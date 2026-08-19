import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Block } from "@blocknote/core";
import { TitleBar } from "./components/TitleBar";
import { Editor } from "./components/Editor";
import { SettingsPanel } from "./components/SettingsPanel";
import { ShortcutsPanel } from "./components/ShortcutsPanel";
import type { Note, Settings, ThemeName } from "./types/note";
import { makeNote, makeSettings, THEME_NAMES } from "./types/note";
import { deriveTitle } from "./services/title";
import { THEMES, resolveTheme, onSystemThemeChange } from "./services/theme";
import { t } from "./services/i18n";
import {
  loadNotes,
  loadSettings,
  saveNotes,
  saveSettings,
  setWindowBackground,
  setWindowOpacityOverride,
  hideWindow,
  onHide,
  onQuit,
  onOpenSettings,
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

  const notesRef = useRef<Note[]>(notes);
  notesRef.current = notes;
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const errorTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

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
        await saveNotes(list);
      }
      setNotes(list);
      setSettings({ ...makeSettings(), ...loadedSettings });
      setActiveId(list[list.length - 1]?.id ?? null);
      setReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

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
    void setWindowBackground(...THEMES[appliedTheme].rgb);
  }, [appliedTheme]);

  // App-modal overlays (settings, theme picker) keep the window fully opaque
  // while open — translucency is only useful while editing content behind.
  useEffect(() => {
    if (!ready) return;
    void setWindowOpacityOverride(settingsOpen || themePickerOpen || shortcutsOpen);
  }, [ready, settingsOpen, themePickerOpen, shortcutsOpen]);

  /* ---------------- persistence ---------------- */

  const scheduleSave = useCallback(() => {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      saveTimer.current = null;
      saveNotes(notesRef.current).catch(reportSaveError);
    }, SAVE_DEBOUNCE_MS);
  }, [reportSaveError]);

  const flushSave = useCallback(() => {
    if (!saveTimer.current) return;
    clearTimeout(saveTimer.current);
    saveTimer.current = null;
    saveNotes(notesRef.current).catch(reportSaveError);
  }, [reportSaveError]);

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
    (note: Note) => (note.title.trim() ? note.title : deriveTitle(note.blocks)),
    [],
  );

  /* ---------------- quick switching (Ctrl+Tab / Ctrl+Shift+Tab) ------------- */

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      // Deliberately ignore Alt/Meta so Alt+Tab (OS) and Ctrl+Meta+Tab never
      // collide; Shift only flips the direction.
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

  // App-level shortcuts: Ctrl+, opens settings, Ctrl+Shift+T cycles the
  // theme, Ctrl+Shift+/ toggles the shortcut cheatsheet. Capture phase so
  // they work even while the editor has focus.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.altKey || e.metaKey) return;
      if (e.ctrlKey && !e.shiftKey && e.key === ",") {
        e.preventDefault();
        e.stopPropagation();
        setSettingsOpen(true);
        return;
      }
      if (!e.ctrlKey || !e.shiftKey) return;
      const k = e.key.toLowerCase();
      if (k === "t") {
        e.preventDefault();
        e.stopPropagation();
        cycleTheme();
        return;
      }
      // Ctrl+Shift+/ (reports as "?" on US layouts, "/" elsewhere).
      if (e.key === "?" || e.key === "/") {
        e.preventDefault();
        e.stopPropagation();
        setShortcutsOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [cycleTheme]);

  const titles = useMemo(() => {
    const m = new Map<string, string>();
    for (const n of notes) m.set(n.id, titleFor(n));
    return m;
  }, [notes, titleFor]);

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
        titleFor={(n) => titles.get(n.id) ?? ""}
        pinned={settings.alwaysOnTop}
        onSelect={setActiveId}
        onNewNote={createNote}
        themeChoice={settings.theme as ThemeName}
        appliedTheme={appliedTheme}
        onSelectTheme={selectTheme}
        onThemePickerOpenChange={setThemePickerOpen}
        onTogglePin={togglePin}
        onDeleteNote={deleteNote}
        onRenameNote={renameNote}
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenShortcuts={() => setShortcutsOpen(true)}
        onHide={hide}
        onClose={hide}
      />
      <main className="min-h-0 flex-1 overflow-y-auto">
        {activeNote && (
          <Editor
            key={activeNote.id}
            note={activeNote}
            blocknoteTheme={THEMES[appliedTheme].blocknote}
            onChange={updateActiveBlocks}
          />
        )}
      </main>
      <SettingsPanel
        open={settingsOpen}
        settings={settings}
        onClose={() => setSettingsOpen(false)}
        onChanged={persistSettings}
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
