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
  try {
    const raw = localStorage.getItem(LS_NOTES);
    return raw ? (JSON.parse(raw) as Note[]) : [];
  } catch {
    return [];
  }
}

export async function saveNotes(notes: Note[]): Promise<void> {
  if (await isNative()) {
    await Store.SaveNotes(notes);
    return;
  }
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
 * Migrate the data directory. Resolves to "" on success, else an error
 * message (pre-check failures are included).
 */
export async function setDataDir(path: string): Promise<string> {
  if (await isNative()) {
    try {
      await Store.SetDataDir(path);
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

export async function setWindowBackground(r: number, g: number, b: number): Promise<void> {
  if (await isNative()) {
    await WailsWindow.SetBackgroundColour(r, g, b, 255);
  }
}

/* ------------------------------------------------------------------ */
/* Events (Go → frontend)                                              */
/* ------------------------------------------------------------------ */

export function onHide(callback: () => void): void {
  if (native) Events.On("app:hide", callback);
}

export function onQuit(callback: () => void): void {
  if (native) Events.On("app:quit", callback);
}

export function onOpenSettings(callback: () => void): void {
  if (native) Events.On("app:open-settings", callback);
}
