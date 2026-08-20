/**
 * shortcuts.ts — the app's shortcut registry and kbd rendering helpers.
 *
 * Three layers of shortcuts exist, and only some are listed in the
 * cheatsheet (ShortcutsPanel):
 *
 *  - Global: the OS-level toggle hotkey (Go/Win32), user-configurable via
 *    Settings. Shown dynamically from the current settings value.
 *  - App-level: handled by our own keydown listeners (App.tsx). Listed here.
 *  - Editor: BlockNote built-ins. The block shortcuts (Ctrl+Alt+1 …) are
 *    already surfaced as badges in the slash menu, so they are intentionally
 *    NOT listed here — only inline formatting shortcuts are. Ctrl+Enter
 *    (check/uncheck, added by slite) is listed because the slash menu does
 *    not show it.
 */

import { t } from "./i18n";

export type ShortcutGroup = "global" | "app" | "formatting";

export interface ShortcutDef {
  id: string;
  /** One or more key combos; each combo renders as its own <kbd>. */
  keys: string[];
  label: string;
  group: ShortcutGroup;
}

export const SHORTCUT_GROUPS: { id: ShortcutGroup; label: string }[] = [
  { id: "global", label: t.shortcutGroupGlobal },
  { id: "app", label: t.shortcutGroupApp },
  { id: "formatting", label: t.shortcutGroupFormatting },
];

/**
 * The shortcuts shown in the cheatsheet, excluding the global toggle hotkey
 * (that one is dynamic — ShortcutsPanel prepends it from the settings).
 */
export const APP_SHORTCUTS: ShortcutDef[] = [
  // App-level
  { id: "new-note", keys: ["Ctrl+N"], label: t.sNewNote, group: "app" },
  { id: "next-note", keys: ["Ctrl+Tab"], label: t.sNextNote, group: "app" },
  { id: "prev-note", keys: ["Ctrl+Shift+Tab"], label: t.sPrevNote, group: "app" },
  { id: "open-settings", keys: ["Ctrl+,"], label: t.sOpenSettings, group: "app" },
  { id: "cycle-theme", keys: ["Ctrl+Shift+T"], label: t.sCycleTheme, group: "app" },
  { id: "show-shortcuts", keys: ["Ctrl+Shift+/"], label: t.sShowShortcuts, group: "app" },
  // Inline formatting (BlockNote/tiptap built-ins)
  { id: "bold", keys: ["Ctrl+B"], label: t.sBold, group: "formatting" },
  { id: "italic", keys: ["Ctrl+I"], label: t.sItalic, group: "formatting" },
  { id: "strike", keys: ["Ctrl+Shift+S"], label: t.sStrike, group: "formatting" },
  { id: "inline-code", keys: ["Ctrl+E"], label: t.sInlineCode, group: "formatting" },
  { id: "check-item", keys: ["Ctrl+Enter"], label: t.sCheckItem, group: "formatting" },
  { id: "undo", keys: ["Ctrl+Z"], label: t.sUndo, group: "formatting" },
  { id: "redo", keys: ["Ctrl+Y", "Ctrl+Shift+Z"], label: t.sRedo, group: "formatting" },
];

/** Split an accelerator into its key parts for <kbd> rendering. */
export function kbdParts(combo: string): string[] {
  return combo.split("+").filter(Boolean);
}
