import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Block } from "@blocknote/core";
import { TitleBar } from "./components/TitleBar";
import { Editor, type NoteConverter } from "./components/Editor";
import { SettingsPanel } from "./components/SettingsPanel";
import { ShortcutsPanel } from "./components/ShortcutsPanel";
import type { Note, Settings, ThemeName, LanguageName } from "./types/note";
import { makeNote, makeSettings, THEME_NAMES, normalizeLanguage } from "./types/note";
import { TitleCache } from "./services/titleCache";
import { isMac } from "./services/platform";
import { THEMES, resolveTheme, onSystemThemeChange } from "./services/theme";
import { t, useLocale, setLocale, resolveChoice } from "./services/i18n";
import {
  loadNotes,
  loadSettings,
  saveNote,
  deleteNote as deleteNoteFile,
  saveSettings,
  setWindowBackground,
  setWindowOpacityOverride,
  hideWindow,
  onHide,
  onQuit,
  onOpenSettings,
  openMarkdownDialog,
  exportAllMarkdown,
} from "./services/bridge";

const SAVE_DEBOUNCE_MS = 800;

export default function App() {
  const [ready, setReady] = useState(false);
  const [notes, setNotes] = useState<Note[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [settings, setSettings] = useState<Settings>(() => makeSettings());
  const [settingsOpen, setSettingsOpen] = useState(false);
  // Whether the shortcut cheatsheet modal is up (drives the window-opacity lift).
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  // Whether the theme picker popover is up (drives the window-opacity lift).
  const [themePickerOpen, setThemePickerOpen] = useState(false);
  // Whether the language picker popover is up (drives the window-opacity lift).
  const [languagePickerOpen, setLanguagePickerOpen] = useState(false);

  const notesRef = useRef<Note[]>(notes);
  notesRef.current = notes;
  // Snapshot of what the backend has persisted (by reference), used to
  // diff-save: only notes whose blocks/title/updatedAt reference moved get
  // written, ids that vanished get deleted.
  const lastSavedRef = useRef<Map<string, Note>>(new Map());
  // Per-note display-title cache (see services/titleCache.ts): a title is
  // re-derived only when that note's blocks/title reference moves, so editing
  // one note never recomputes every other note's title.
  const titleCacheRef = useRef<TitleCache | null>(null);
  if (titleCacheRef.current === null) {
    titleCacheRef.current = new TitleCache();
  }
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const errorTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  // Markdown converter bound to the active note's editor (same schema for
  // every note), used for export/import. Null while the editor is mounting.
  const converterRef = useRef<NoteConverter | null>(null);

  // Show a dismissible banner when persistence fails (disk full, permission…)
  // and auto-clear it after a few seconds.
  const reportSaveError = useCallback((e: unknown) => {
    console.error(t.saveFailed, e);
    setSaveError(String((e as Error)?.message ?? e));
    if (errorTimer.current) clearTimeout(errorTimer.current);
    errorTimer.current = setTimeout(() => setSaveError(null), 4000);
  }, []);

  /* ---------------- boot ---------------- */

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [loadedNotes, loadedSettings] = await Promise.all([
        loadNotes(),
        loadSettings(),
      ]);
      if (cancelled) return;
      let list = loadedNotes;
      if (list.length === 0) {
        // First run: seed one empty note so the editor has a home.
        list = [makeNote()];
        await saveNote(list[0]);
      }
      lastSavedRef.current = new Map(list.map((n) => [n.id, n]));
      setNotes(list);
      // Normalize a legacy/empty language ("") to "system" before storing.
      const baseSettings = { ...makeSettings(), ...loadedSettings };
      baseSettings.language = normalizeLanguage(baseSettings.language);
      setSettings(baseSettings);
      // Resolve the UI language once at startup: follow the OS unless the
      // user previously picked a concrete locale (see resolveChoice).
      setLocale(resolveChoice(baseSettings.language));
      setActiveId(list[list.length - 1]?.id ?? null);
      setReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  /* ---------------- language ---------------- */

  // Re-render the tree on setLocale so components reading `t` pick up the
  // new language (t is a live proxy; the subscription just triggers updates).
  // Also used to remount the editor so BlockNote's locale dictionary applies.
  const locale = useLocale();

  /* ---------------- theme side effects ---------------- */

  // Bump when the OS dark-mode preference changes so "system" re-resolves.
  const [themeTick, setThemeTick] = useState(0);
  useEffect(() => onSystemThemeChange(() => setThemeTick((t) => t + 1)), []);

  const appliedTheme = useMemo(() => {
    const choice = (THEME_NAMES.includes(settings.theme as ThemeName)
      ? settings.theme
      : "system") as ThemeName;
    return resolveTheme(choice);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings.theme, themeTick]);

  useEffect(() => {
    document.documentElement.dataset.theme = appliedTheme;
    // macOS: the window shell is transparent (Backdrop=Transparent) and the
    // note background carries the alpha — pass it through so the window
    // background matches the CSS. Windows ignores the alpha channel of
    // SetBackgroundColour (its whole-window alpha is applied natively).
    // Clamp the alpha to the opacity floor: a persisted 0 (e.g. a legacy
    // settings file) would otherwise make the very first frame see-through.
    const alpha = isMac() ? Math.round(Math.max(0.3, settings.opacity) * 255) : 255;
    void setWindowBackground(...THEMES[appliedTheme].rgb, alpha);
  }, [appliedTheme, settings.opacity]);

  // macOS note-background translucency: the CSS background (body + title bar)
  // is mixed with transparency via --bg-opacity. Overlays (settings, theme
  // picker, shortcuts) force 100% — matching the Windows opacity override.
  // Clamped to the opacity floor (30%) so a stale/zero opacity value can
  // never blank the whole window.
  useEffect(() => {
    const opaque = settingsOpen || themePickerOpen || languagePickerOpen || shortcutsOpen || !isMac();
    const pct = opaque ? 100 : Math.max(30, Math.round(settings.opacity * 100));
    document.documentElement.style.setProperty("--bg-opacity", `${pct}%`);
  }, [settings.opacity, settingsOpen, themePickerOpen, shortcutsOpen]);

  // App-modal overlays (settings, theme picker) keep the window fully opaque
  // while open — translucency is only useful while editing content behind.
  useEffect(() => {
    if (!ready) return;
    void setWindowOpacityOverride(settingsOpen || themePickerOpen || languagePickerOpen || shortcutsOpen);
  }, [ready, settingsOpen, themePickerOpen, shortcutsOpen]);

  /* ---------------- persistence ---------------- */

  // Persist only what changed since the last successful save. React's
  // immutable updates give us fresh references on every edit, so reference
  // comparison is a reliable dirty marker (no timestamp-collision risk).
  const persistChanges = useCallback(async () => {
    const current = notesRef.current;
    const last = lastSavedRef.current;
    const next = new Map<string, Note>();
    const dirty: Note[] = [];
    for (const n of current) {
      next.set(n.id, n);
      const prev = last.get(n.id);
      if (!prev || prev.blocks !== n.blocks || prev.title !== n.title || prev.updatedAt !== n.updatedAt) {
        dirty.push(n);
      }
    }
    try {
      for (const id of last.keys()) {
        if (!next.has(id)) await deleteNoteFile(id);
      }
      for (const n of dirty) await saveNote(n);
      lastSavedRef.current = next;
    } catch (err) {
      reportSaveError(err);
    }
  }, [reportSaveError]);

  const scheduleSave = useCallback(() => {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      saveTimer.current = null;
      void persistChanges();
    }, SAVE_DEBOUNCE_MS);
  }, [persistChanges]);

  const flushSave = useCallback(() => {
    if (!saveTimer.current) return;
    clearTimeout(saveTimer.current);
    saveTimer.current = null;
    void persistChanges();
  }, [persistChanges]);

  // Flush pending saves when the window hides or the app quits.
  useEffect(() => {
    onHide(flushSave);
    onQuit(flushSave);
    window.addEventListener("beforeunload", flushSave);
    return () => window.removeEventListener("beforeunload", flushSave);
  }, [flushSave]);

  /* ---------------- actions ---------------- */

  const updateActiveBlocks = useCallback(
    (blocks: Block[]) => {
      setNotes((prev) => {
        const next = prev.map((n) =>
          n.id === activeId
            ? { ...n, blocks: blocks as unknown as Note["blocks"], updatedAt: Date.now() }
            : n,
        );
        notesRef.current = next;
        return next;
      });
      scheduleSave();
    },
    [activeId, scheduleSave],
  );

  const createNote = useCallback(() => {
    const note = makeNote();
    const next = [...notesRef.current, note];
    notesRef.current = next;
    setNotes(next);
    setActiveId(note.id);
    scheduleSave();
  }, [scheduleSave]);

  const deleteNote = useCallback(
    (id: string) => {
      const prev = notesRef.current;
      const next = prev.filter((n) => n.id !== id);
      titleCacheRef.current?.delete(id);
      if (next.length === 0) {
        // Keep at least one note so the editor always has a home.
        next.push(makeNote());
      }
      notesRef.current = next;
      setNotes(next);
      if (activeId === id) {
        setActiveId(next[next.length - 1]?.id ?? null);
      }
      scheduleSave();
    },
    [activeId, scheduleSave],
  );

  // Manual title override: "" restores the derived first-line title.
  const renameNote = useCallback(
    (id: string, title: string) => {
      const prev = notesRef.current;
      const existing = prev.find((n) => n.id === id);
      if (!existing || existing.title === title) return;
      const next = prev.map((n) =>
        n.id === id ? { ...n, title, updatedAt: Date.now() } : n,
      );
      notesRef.current = next;
      setNotes(next);
      scheduleSave();
    },
    [scheduleSave],
  );

  const persistSettings = useCallback(
    (next: Settings) => {
      setSettings(next);
      saveSettings(next).catch(reportSaveError);
    },
    [reportSaveError],
  );

  // Tray menu → Settings… opens the panel (and the panel itself closes via ✕).
  useEffect(() => {
    onOpenSettings(() => setSettingsOpen(true));
  }, []);

  const selectTheme = useCallback(
    (choice: ThemeName) => {
      persistSettings({ ...settings, theme: choice });
    },
    [persistSettings, settings],
  );

  // Language choice: resolve + apply immediately, persist for next launch.
  // After a manual pick the UI no longer follows the OS language.
  const selectLanguage = useCallback(
    (choice: LanguageName) => {
      setLocale(resolveChoice(choice));
      persistSettings({ ...settings, language: choice });
    },
    [persistSettings, settings],
  );

  // Ctrl+Shift+T: cycle system → dark → gray → yellow → system.
  const cycleTheme = useCallback(() => {
    setSettings((prev) => {
      const idx = THEME_NAMES.indexOf(prev.theme as ThemeName);
      const next = THEME_NAMES[(idx + 1) % THEME_NAMES.length];
      const s = { ...prev, theme: next };
      saveSettings(s).catch(reportSaveError);
      return s;
    });
  }, [reportSaveError]);

  const togglePin = useCallback(() => {
    setSettings((prev) => {
      const next = { ...prev, alwaysOnTop: !prev.alwaysOnTop };
      saveSettings(next).catch(reportSaveError);
      return next;
    });
  }, [reportSaveError]);

  const hide = useCallback(() => {
    flushSave();
    void hideWindow();
  }, [flushSave]);

  const titleFor = useCallback(
    (note: Note) => titleCacheRef.current!.titleFor(note),
    [],
  );

  /* ---------------- markdown export / import ---------------- */

  // Single note → .md into a user-picked folder (defaulting to Downloads),
  // named after the note title. Uses the persisted blocks (App state) rather
  // than the editor's live document, so it stays correct even while a
  // debounced save is pending.
  const exportNoteMarkdown = useCallback(
    async (note: Note) => {
      const converter = converterRef.current;
      if (!converter) return;
      const name = (titleFor(note) || t.untitled).trim() || "Untitled";
      await exportAllMarkdown([
        { name, content: converter.blocksToMarkdown(note.blocks as Block[]) },
      ]).catch(reportSaveError);
    },
    [reportSaveError, titleFor],
  );

  // .md file → new note (appended after the active note).
  const importNoteMarkdown = useCallback(async () => {
    const converter = converterRef.current;
    if (!converter) return;
    const md = await openMarkdownDialog().catch(() => "");
    if (!md) return; // cancelled or empty file
    const note = makeNote();
    note.blocks = converter.markdownToBlocks(md) as unknown as Note["blocks"];
    const next = [...notesRef.current, note];
    notesRef.current = next;
    setNotes(next);
    setActiveId(note.id);
    scheduleSave();
  }, [scheduleSave]);

  // All notes → one .md per note into a user-picked folder. Returns the
  // number of files written so Settings can show feedback.
  const exportAllNotesMarkdown = useCallback(async (): Promise<number> => {
    const converter = converterRef.current;
    if (!converter) return 0;
    const files = notesRef.current.map((n) => ({
      name: (titleFor(n) || t.untitled).trim() || "Untitled",
      content: converter.blocksToMarkdown(n.blocks as Block[]),
    }));
    try {
      return await exportAllMarkdown(files);
    } catch (e) {
      reportSaveError(e);
      return 0;
    }
  }, [reportSaveError, titleFor]);

  /* ---------------- quick switching (Ctrl+Tab / Cmd+Shift+[ ]) ------------- */

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isMac()) {
        // macOS: Cmd+Shift+] = next, Cmd+Shift+[ = prev (browser tab idiom).
        // Ctrl+Tab is the system keyboard-navigation combo and Cmd+Tab the
        // app switcher, so both are deliberately excluded.
        if (!e.metaKey || e.ctrlKey || e.altKey || !e.shiftKey) return;
        if (e.code !== "BracketRight" && e.code !== "BracketLeft") return;
        const list = notesRef.current;
        if (list.length < 2) return;
        let idx = list.findIndex((n) => n.id === activeId);
        if (idx < 0) idx = 0;
        const dir = e.code === "BracketRight" ? 1 : -1;
        const next = list[(idx + dir + list.length) % list.length];
        e.preventDefault();
        e.stopPropagation();
        setActiveId(next.id);
        return;
      }
      // Windows/Linux: Ctrl+Tab / Ctrl+Shift+Tab. Deliberately ignore
      // Alt/Meta so Alt+Tab (OS) and Ctrl+Meta+Tab never collide; Shift only
      // flips the direction.
      if (!e.ctrlKey || e.altKey || e.metaKey) return;
      if (e.key !== "Tab") return;
      const list = notesRef.current;
      if (list.length < 2) return;
      let idx = list.findIndex((n) => n.id === activeId);
      if (idx < 0) idx = 0;
      const dir = e.shiftKey ? -1 : 1;
      const next = list[(idx + dir + list.length) % list.length];
      // Capture phase + stopPropagation: BlockNote may also handle Tab;
      // the combo must switch notes even while the editor has focus.
      e.preventDefault();
      e.stopPropagation();
      setActiveId(next.id);
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [activeId]);

  // App-level shortcuts (Mod-mapped: Cmd on macOS, Ctrl elsewhere): new
  // note, open settings, cycle theme, toggle the shortcut cheatsheet.
  // Capture phase so they work even while the editor has focus.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const mod = isMac() ? e.metaKey : e.ctrlKey;
      // The other platform's modifier must stay out: Cmd on mac, Ctrl on win.
      const foreign = isMac() ? e.ctrlKey : e.metaKey;
      if (!mod || foreign || e.altKey) return;
      if (!e.shiftKey && e.key.toLowerCase() === "n") {
        e.preventDefault();
        e.stopPropagation();
        createNote();
        return;
      }
      if (!e.shiftKey && e.key === ",") {
        e.preventDefault();
        e.stopPropagation();
        setSettingsOpen(true);
        return;
      }
      if (!e.shiftKey) return;
      const k = e.key.toLowerCase();
      if (k === "t") {
        e.preventDefault();
        e.stopPropagation();
        cycleTheme();
        return;
      }
      // Mod+Shift+/ (reports as "?" on US layouts, "/" elsewhere).
      if (e.key === "?" || e.key === "/") {
        e.preventDefault();
        e.stopPropagation();
        setShortcutsOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [cycleTheme, createNote]);

  const activeNote = notes.find((n) => n.id === activeId) ?? null;

  /* ---------------- render ---------------- */

  if (!ready) {
    return <div className="h-full w-full" aria-busy="true" />;
  }

  return (
    <div className="flex h-full flex-col">
      <TitleBar
        notes={notes}
        activeId={activeId}
        titleFor={titleFor}
        pinned={settings.alwaysOnTop}
        language={settings.language as LanguageName}
        onSelectLanguage={selectLanguage}
        onLanguagePickerOpenChange={setLanguagePickerOpen}
        onSelect={setActiveId}
        onNewNote={createNote}
        themeChoice={settings.theme as ThemeName}
        appliedTheme={appliedTheme}
        onSelectTheme={selectTheme}
        onThemePickerOpenChange={setThemePickerOpen}
        onTogglePin={togglePin}
        onDeleteNote={deleteNote}
        onRenameNote={renameNote}
        onExportNote={exportNoteMarkdown}
        onImportNote={() => void importNoteMarkdown()}
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenShortcuts={() => setShortcutsOpen(true)}
        onHide={hide}
        onClose={hide}
      />
      <main className="min-h-0 flex-1 overflow-y-auto">
        {activeNote && (
          <Editor
            key={`${activeNote.id}:${locale}`}
            note={activeNote}
            blocknoteTheme={THEMES[appliedTheme].blocknote}
            onChange={updateActiveBlocks}
            onConverterReady={(c) => (converterRef.current = c)}
          />
        )}
      </main>
      <SettingsPanel
        open={settingsOpen}
        settings={settings}
        onClose={() => setSettingsOpen(false)}
        onChanged={persistSettings}
        onExportAll={exportAllNotesMarkdown}
      />
      <ShortcutsPanel
        open={shortcutsOpen}
        hotkey={settings.hotkey}
        onClose={() => setShortcutsOpen(false)}
      />
      {saveError && (
        <div className="pointer-events-none fixed inset-x-0 bottom-2 z-[110] flex justify-center">
          <div
            role="alert"
            className="rounded-md bg-red-600 px-3 py-1.5 text-[11px] font-medium text-white shadow-lg"
          >
            {t.saveFailed}: {saveError}
          </div>
        </div>
      )}
    </div>
  );
}
