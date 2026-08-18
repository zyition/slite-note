/**
 * Centralized UI strings, English-first (per decision). Adding a locale means
 * replacing this object with a map keyed by locale — the keys stay stable.
 */

const en = {
  appName: "slite-note",
  untitled: "Untitled",
  newNote: "New note",
  notes: "Notes",
  alwaysOnTop: "Always on top",
  theme: "Theme",
  hide: "Hide",
  close: "Hide to tray",
  settings: "Settings",
  placeholder: "Type '/' for commands…",
  saveFailed: "Failed to save note",
  loadFailed: "Failed to load notes",

  // Note picker: delete
  deleteNote: "Delete note",
  deleteConfirm: "Delete?",

  // Settings panel
  settingsTitle: "Settings",
  closePanel: "Close settings",
  hotkeySection: "Global shortcut",
  hotkeyDesc: "Press the key combo that shows/hides the window from anywhere.",
  changeHotkey: "Change",
  pressNewHotkey: "Press the new shortcut…",
  hotkeyChangeFailed: "Could not set that shortcut",
  autoStartSection: "Launch at startup",
  autoStartDesc: "Start slite automatically when you sign in to Windows.",
  dataSection: "Data location",
  dataDesc: "Notes live in a single notes.json file in this folder.",
  openExplorer: "Open in Explorer",
  changeLocationTitle: "Move data to another folder",
  moveData: "Change location…",
  migrateDone: "Done — notes moved, slite now uses the new folder.",
  nativeOnly: "Only available in the desktop app.",
  cancel: "Cancel",
  opacitySection: "Window opacity",
  opacityDesc: "Make the window see-through. 100% is fully opaque.",

  // About section
  aboutSection: "About",
  versionLabel: "Version",
  homepageLabel: "Homepage",
  licenseLabel: "License",
  aboutDesc:
    "slite-note = slide + lite. A minimal sticky note that glides in and out. Built with Wails, BlockNote, React, Tailwind and lucide.",
};

export const t: typeof en = en;
