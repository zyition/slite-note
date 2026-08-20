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
  themeFollowsOs: "Follows the OS",
  hide: "Hide",
  close: "Hide to tray",
  settings: "Settings",
  placeholder: "Type '/' for commands…",
  saveFailed: "Failed to save note",
  loadFailed: "Failed to load notes",

  // Note picker: delete / rename
  deleteNote: "Delete note",
  deleteConfirm: "Delete?",
  renameNote: "Rename note",
  exportNote: "Export as Markdown",
  importMarkdown: "Import Markdown…",
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
  dataDesc:
    "Notes and settings are stored in this folder. Move them to a new location, or point slite at an existing one.",
  openExplorer: "Open in Explorer",
  changeLocationTitle: "Move data to another folder",
  moveData: "Change location…",
  migrateDone: "Done — notes moved, slite now uses the new folder.",
  useExisting: "Use existing…",
  useExistingDone: "Done — slite now reads notes from the selected folder.",
  nativeOnly: "Only available in the desktop app.",
  cancel: "Cancel",
  opacitySection: "Window opacity",
  opacityDesc: "Make the window see-through. 100% is fully opaque.",

  // Markdown export (settings)
  exportAllSection: "Markdown export",
  exportAllDesc: "Write every note as its own .md file into a folder of your choice.",
  exportAll: "Export all as Markdown…",
  exportDone: (n: number) => `Exported ${n} note${n === 1 ? "" : "s"} to the selected folder.`,

  // Shortcut cheatsheet
  shortcutsTitle: "Keyboard shortcuts",
  shortcutGroupGlobal: "Global",
  shortcutGroupApp: "App",
  shortcutGroupFormatting: "Formatting",
  sToggleWindow: "Show / Hide slite",
  sNextNote: "Next note",
  sPrevNote: "Previous note",
  sNewNote: "New note",
  sOpenSettings: "Open settings",
  sCycleTheme: "Cycle theme",
  sShowShortcuts: "Show shortcut cheatsheet",
  sBold: "Bold",
  sItalic: "Italic",
  sStrike: "Strikethrough",
  sInlineCode: "Inline code",
  sCheckItem: "Toggle checkbox",
  sUndo: "Undo",
  sRedo: "Redo",

  // About section
  aboutSection: "About",
  versionLabel: "Version",
  homepageLabel: "Homepage",
  licenseLabel: "License",
  aboutDesc:
    "slite-note. A minimal sticky note for Windows. Built with Wails, BlockNote, React, Tailwind and lucide.",
};

export const t: typeof en = en;
