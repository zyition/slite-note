import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Block } from "@blocknote/core";
import { TitleBar } from "./components/TitleBar";
import { Editor } from "./components/Editor";
import { SettingsPanel } from "./components/SettingsPanel";
import type { Note, Settings, ThemeName } from "./types/note";
import { makeNote, makeSettings, THEME_NAMES } from "./types/note";
import { deriveTitle } from "./services/title";
import { THEMES, resolveTheme, cycleTheme as nextThemeName, onSystemThemeChange } from "./services/theme";
import { t } from "./services/i18n";
import {
  loadNotes,
  loadSettings,
  saveNotes,
  saveSettings,
  setWindowBackground,
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

  const notesRef = useRef<Note[]>(notes);
  notesRef.current = notes;
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  /* ---------------- persistence ---------------- */

  const scheduleSave = useCallback(() => {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      saveTimer.current = null;
      saveNotes(notesRef.current).catch((e) => console.error(t.saveFailed, e));
    }, SAVE_DEBOUNCE_MS);
  }, []);

  const flushSave = useCallback(() => {
    if (!saveTimer.current) return;
    clearTimeout(saveTimer.current);
    saveTimer.current = null;
    saveNotes(notesRef.current).catch((e) => console.error(t.saveFailed, e));
  }, []);

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

  const persistSettings = useCallback((next: Settings) => {
    setSettings(next);
    saveSettings(next).catch((e) => console.error(t.saveFailed, e));
  }, []);

  // Tray menu → Settings… opens the panel (and the panel itself closes via ✕).
  useEffect(() => {
    onOpenSettings(() => setSettingsOpen(true));
  }, []);

  const cycleTheme = useCallback(() => {
    setSettings((prev) => {
      const next = { ...prev, theme: nextThemeName(prev.theme as ThemeName) };
      saveSettings(next).catch((e) => console.error(t.saveFailed, e));
      return next;
    });
  }, []);

  const togglePin = useCallback(() => {
    setSettings((prev) => {
      const next = { ...prev, alwaysOnTop: !prev.alwaysOnTop };
      saveSettings(next).catch((e) => console.error(t.saveFailed, e));
      return next;
    });
  }, []);

  const hide = useCallback(() => {
    flushSave();
    void hideWindow();
  }, [flushSave]);

  const titleFor = useCallback(
    (note: Note) => (note.title.trim() ? note.title : deriveTitle(note.blocks)),
    [],
  );

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
        onCycleTheme={cycleTheme}
        onTogglePin={togglePin}
        onDeleteNote={deleteNote}
        onOpenSettings={() => setSettingsOpen(true)}
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
    </div>
  );
}
