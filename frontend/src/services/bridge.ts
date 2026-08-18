/**
 * bridge.ts — the storageAdapter / windowAdapter from req.md.
 *
 * Detects whether we run inside the Wails runtime (native) or in a plain
 * browser (debugging the UI standalone). In native mode every call goes
 * through the generated Go bindings and the Wails runtime window API; in
 * browser mode we fall back to localStorage and no-op window controls.
 */
import { Events, Window as WailsWindow } from "@wailsio/runtime";
import { Store } from "../../bindings/slite";
import type { Note, Settings } from "../../bindings/slite";

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
    return raw
      ? (JSON.parse(raw) as Settings)
      : { theme: "yellow", alwaysOnTop: false };
  } catch {
    return { theme: "yellow", alwaysOnTop: false };
  }
}

export async function saveSettings(settings: Settings): Promise<void> {
  if (await isNative()) {
    await Store.SaveSettings(settings);
    return;
  }
  localStorage.setItem(LS_SETTINGS, JSON.stringify(settings));
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
