/**
 * bridge.ts — the storageAdapter / windowAdapter from req.md.
 *
 * Detects whether we run inside the Wails runtime (native) or in a plain
 * browser (debugging the UI standalone). In native mode every call goes
 * through the generated Go bindings and the Wails runtime window API; in
 * browser mode we fall back to localStorage and no-op window controls.
 */
import { Events, Window as WailsWindow } from "@wailsio/runtime";
import { Store } from "../../bindings/github.com/zyition/slite-note";
import type { Note, Settings } from "../../bindings/github.com/zyition/slite-note";
import { makeSettings } from "../types/note";
import { t } from "./i18n";

const LS_NOTES = "slite:notes";
const LS_SETTINGS = "slite:settings";

let native: boolean | null = null;

/** Probe the Go backend with a call that never fails in native mode. */
async function isNative(): Promise<boolean> {
  if (native !== null) return native;
  try {
    await Store.Ping();
    native = true;
  } catch {
    native = false;
  }
  return native;
}

/* ------------------------------------------------------------------ */
/* Storage adapter                                                     */
/* ------------------------------------------------------------------ */

export async function loadNotes(): Promise<Note[]> {
  if (await isNative()) {
    return (await Store.LoadNotes()) ?? [];
  }
  return loadNotesFromLocalStorage();
}

function loadNotesFromLocalStorage(): Note[] {
  try {
    const raw = localStorage.getItem(LS_NOTES);
    return raw ? (JSON.parse(raw) as Note[]) : [];
  } catch {
    return [];
  }
}

// Per-note persistence in native mode: editing a note rewrites only its own
// file. The browser fallback keeps a whole-list localStorage write (no file
// concerns there), which is what diff-save collapses to anyway.
export async function saveNote(note: Note): Promise<void> {
  if (await isNative()) {
    await Store.SaveNote(note);
    return;
  }
  const notes = loadNotesFromLocalStorage();
  const idx = notes.findIndex((n) => n.id === note.id);
  if (idx >= 0) notes[idx] = note;
  else notes.push(note);
  localStorage.setItem(LS_NOTES, JSON.stringify(notes));
}

export async function deleteNote(id: string): Promise<void> {
  if (await isNative()) {
    await Store.DeleteNote(id);
    return;
  }
  const notes = loadNotesFromLocalStorage().filter((n) => n.id !== id);
  localStorage.setItem(LS_NOTES, JSON.stringify(notes));
}

export async function loadSettings(): Promise<Settings> {
  if (await isNative()) {
    return await Store.LoadSettings();
  }
  try {
    const raw = localStorage.getItem(LS_SETTINGS);
    return raw ? (JSON.parse(raw) as Settings) : makeSettings();
  } catch {
    return makeSettings();
  }
}

export async function saveSettings(settings: Settings): Promise<void> {
  if (await isNative()) {
    await Store.SaveSettings(settings);
    return;
  }
  localStorage.setItem(LS_SETTINGS, JSON.stringify(settings));
}

/** Re-register the global toggle hotkey (no-op in browser fallback). */
export async function setHotkey(combo: string): Promise<void> {
  if (await isNative()) {
    await Store.SetHotkey(combo);
  }
}

/** Suspend the global toggle hotkey while recording a new one. */
export async function suspendHotkey(): Promise<void> {
  if (await isNative()) {
    await Store.SuspendHotkey();
  }
}

/** Restore the toggle hotkey after a recording session. */
export async function resumeHotkey(): Promise<void> {
  if (await isNative()) {
    await Store.ResumeHotkey();
  }
}

/** Native folder picker; resolves to "" when the user cancels. */
export async function chooseDataDir(): Promise<string> {
  if (await isNative()) {
    return await Store.ChooseDataDir();
  }
  throw new Error(t.nativeOnly);
}

/** Open the active data directory in Explorer (native only). */
export async function openDataDir(): Promise<void> {
  if (await isNative()) {
    await Store.OpenDataDir();
    return;
  }
  throw new Error(t.nativeOnly);
}

/** App version for the About section ("dev" in browser fallback). */
export async function appVersion(): Promise<string> {
  if (await isNative()) {
    return await Store.AppVersion();
  }
  return "dev";
}

/** Open a URL in the default browser (native) or a new tab (fallback). */
export async function openUrl(url: string): Promise<void> {
  if (await isNative()) {
    await Store.OpenURL(url);
    return;
  }
  window.open(url, "_blank", "noopener");
}

/** Active data directory path (native); a friendly label in fallback mode. */
export async function currentDataDir(): Promise<string> {
  if (await isNative()) {
    return await Store.CurrentDataDir();
  }
  return "localStorage (browser)";
}

/* ------------------------------------------------------------------ */
/* Markdown import / export                                             */
/* ------------------------------------------------------------------ */

/**
 * Pick and read a markdown/text file via the native open dialog. Resolves to
 * the file content, or "" when the user cancels (or the file is empty).
 * Browser fallback uses a hidden <input type="file">.
 */
export async function openMarkdownDialog(): Promise<string> {
  if (await isNative()) {
    return await Store.OpenMarkdownDialog();
  }
  return new Promise<string>((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".md,.markdown,.txt";
    input.onchange = () => {
      const file = input.files?.[0];
      if (!file) return resolve("");
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result ?? ""));
      reader.onerror = () => resolve("");
      reader.readAsText(file);
    };
    input.click();
  });
}

/**
 * Export every note as its own .md file into a user-picked folder
 * (defaulting to Downloads in native mode). Resolves to the number of files
 * written (0 when the user cancels). Single-note export passes a
 * one-element array. Browser fallback downloads each file via an anchor.
 */
export async function exportAllMarkdown(
  files: { name: string; content: string }[],
): Promise<number> {
  if (await isNative()) {
    return await Store.ExportAllMarkdown(files);
  }
  for (const f of files) {
    const blob = new Blob([f.content], { type: "text/markdown;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${f.name}.md`;
    a.click();
    URL.revokeObjectURL(url);
  }
  return files.length;
}

/**
 * Pre-migration check for a candidate data directory. Resolves to "" when
 * the target passes, otherwise a human-readable error message.
 */
export async function validateDataDir(path: string): Promise<string> {
  if (await isNative()) {
    try {
      await Store.ValidateDataDir(path);
      return "";
    } catch (e) {
      return String((e as Error)?.message ?? e);
    }
  }
  return t.nativeOnly;
}

/**
 * Migrate the data directory (copy data, switch, remove old). Resolves to ""
 * on success, else an error message (pre-check failures are included).
 */
export async function moveDataDir(path: string): Promise<string> {
  if (await isNative()) {
    try {
      await Store.MoveDataDir(path);
      return "";
    } catch (e) {
      return String((e as Error)?.message ?? e);
    }
  }
  return t.nativeOnly;
}

/**
 * Adopt an existing data directory (point at it, reload preferences; no copy
 * or delete). Resolves to "" on success, else an error message.
 */
export async function useDataDir(path: string): Promise<string> {
  if (await isNative()) {
    try {
      await Store.UseDataDir(path);
      return "";
    } catch (e) {
      return String((e as Error)?.message ?? e);
    }
  }
  return t.nativeOnly;
}

/* ------------------------------------------------------------------ */
/* Window adapter                                                      */
/* ------------------------------------------------------------------ */

export async function hideWindow(): Promise<void> {
  if (await isNative()) {
    await WailsWindow.Hide();
  } else {
    console.log("[slite] window hide (browser fallback: no-op)");
  }
}

export async function setAlwaysOnTop(on: boolean): Promise<void> {
  if (await isNative()) {
    await WailsWindow.SetAlwaysOnTop(on);
  } else {
    console.log("[slite] setAlwaysOnTop", on, "(browser fallback: no-op)");
  }
}

/**
 * Set the native window background colour. macOS honours the alpha channel
 * (the note background is translucent via CSS, so the shell colour just needs
 * to match); Windows ignores alpha (whole-window opacity is applied natively
 * via WS_EX_LAYERED). Default alpha 255 = opaque.
 */
export async function setWindowBackground(
  r: number,
  g: number,
  b: number,
  alpha = 255,
): Promise<void> {
  if (await isNative()) {
    await WailsWindow.SetBackgroundColour(r, g, b, alpha);
  }
}

/**
 * While an app-modal overlay is open (settings panel, theme picker) the
 * window must not stay translucent: the user is not editing, and a
 * see-through UI over a dimmed backdrop is messy. Forces the window fully
 * opaque until released (no-op in browser fallback).
 */
export async function setWindowOpacityOverride(on: boolean): Promise<void> {
  if (await isNative()) {
    await Store.SetWindowOpacityOverride(on);
  }
}

/* ------------------------------------------------------------------ */
/* Events (Go → frontend)                                              */
/* ------------------------------------------------------------------ */

export function onHide(callback: () => void): void {
  if (native) Events.On("app:hide", callback);
}

/** Window shown (hotkey / tray) — the editor focuses the end for typing. */
export function onShow(callback: () => void): void {
  if (native) Events.On("app:show", callback);
}

export function onQuit(callback: () => void): void {
  if (native) Events.On("app:quit", callback);
}

export function onOpenSettings(callback: () => void): void {
  if (native) Events.On("app:open-settings", callback);
}
