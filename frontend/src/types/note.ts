/**
 * Domain types (mirror of the Go bindings in ../bindings/github.com/zyition/slite-note/models.ts).
 * Kept here so components don't import generated bindings directly and the
 * browser fallback can construct the same shapes from localStorage.
 */
import type { Note, Settings } from "../../bindings/github.com/zyition/slite-note";

export type { Note, Settings };

export type ThemeName = "system" | "yellow" | "gray" | "dark";

// All four are user-selectable: follow the OS (which resolves to dark/gray),
// or force one of the concrete themes. Order = theme-picker order.
export const THEME_NAMES: ThemeName[] = ["system", "dark", "gray", "yellow"];

/** UUID with a fallback for insecure origins (crypto.randomUUID needs a
 * secure context; the browser-fallback mode may run on plain http). */
export function uuid(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function makeNote(): Note {
  const now = Date.now();
  return {
    id: uuid(),
    title: "",
    blocks: [],
    createdAt: now,
    updatedAt: now,
  };
}

export function makeSettings(partial?: Partial<Settings>): Settings {
  return {
    theme: "system",
    alwaysOnTop: false,
    hotkey: "Alt+Shift+S",
    launchAtStartup: false,
    opacity: 1,
    windowX: 0,
    windowY: 0,
    windowWidth: 0,
    windowHeight: 0,
    ...partial,
  };
}
