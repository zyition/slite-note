/**
 * Centralized UI strings, English-first (per decision). Adding a locale means
 * replacing this object with a map keyed by locale — the keys stay stable.
 */

const en = {
  appName: "slite",
  untitled: "Untitled",
  newNote: "New note",
  notes: "Notes",
  alwaysOnTop: "Always on top",
  theme: "Theme",
  hide: "Hide",
  close: "Hide to tray",
  placeholder: "Type '/' for commands…",
  saveFailed: "Failed to save note",
  loadFailed: "Failed to load notes",
};

export const t: typeof en = en;
