/**
 * Domain types (mirror of the Go bindings in ../bindings/slite/models.ts).
 * Kept here so components don't import generated bindings directly and the
 * browser fallback can construct the same shapes from localStorage.
 */
import type { Note, Settings } from "../../bindings/slite";

export type { Note, Settings };

export type ThemeName = "system" | "yellow" | "gray" | "dark";

// The only user-facing choices: follow the OS, or force the yellow sticky.
// ("gray"/"dark" remain valid legacy values for persisted settings.)
export const THEME_NAMES: ThemeName[] = ["system", "yellow"];

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
    dataDir: "",
    opacity: 1,
    ...partial,
  };
}
