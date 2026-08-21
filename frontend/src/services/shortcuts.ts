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
 *
 * Key combos are Mod-mapped: the primary modifier is Cmd on macOS and Ctrl
 * elsewhere, matching BlockNote's own "Mod" convention. Keys that clash with
 * macOS system shortcuts are overridden individually — e.g. note switching
 * uses Cmd+Shift+[ / Cmd+Shift+] on macOS because Ctrl+Tab is the system
 * keyboard-navigation combo and Cmd+Tab is the app switcher (ADR-0007).
 */

import { isMac } from "./platform";
import { t } from "./i18n";

export type ShortcutGroup = "global" | "app" | "formatting";

export interface ShortcutDef {
  id: string;
  /** One or more key combos; each combo renders as its own <kbd>. */
  keys: string[];
  label: string;
  group: ShortcutGroup;
}

/**
 * Cheatsheet groups in render order. The display label is NOT cached here:
 * it is resolved per-render from the live `t` proxy (ShortcutsPanel maps
 * group.id → t.shortcutGroup*), so switching language re-renders correctly.
 * (A module-level label constant would capture the English strings once at
 * import time — the i18n bug fixed here.)
 */
export const SHORTCUT_GROUPS: ShortcutGroup[] = ["global", "app", "formatting"];

/** The primary modifier token of the platform (Wails accelerator syntax). */
const MOD = isMac() ? "Cmd" : "Ctrl";

/** The per-platform combos for each shortcut id. */
const SHORTCUT_KEYS: Record<string, string[]> = {
  "new-note": [`${MOD}+N`],
  // macOS: Ctrl+Tab is the system keyboard-navigation combo, Cmd+Tab the app
  // switcher — note switching moves to the browser "previous/next tab" pair.
  "next-note": isMac() ? ["Cmd+Shift+]"] : ["Ctrl+Tab"],
  "prev-note": isMac() ? ["Cmd+Shift+["] : ["Ctrl+Shift+Tab"],
  "open-settings": [`${MOD}+,`],
  "cycle-theme": [`${MOD}+Shift+T`],
  "show-shortcuts": [`${MOD}+Shift+/`],
  // Inline formatting — BlockNote's built-ins already use Mod (⌘ on mac), so
  // the cheatsheet just mirrors what the editor actually binds.
  bold: [`${MOD}+B`],
  italic: [`${MOD}+I`],
  strike: [`${MOD}+Shift+S`],
  "inline-code": [`${MOD}+E`],
  "check-item": [`${MOD}+Enter`],
  undo: [`${MOD}+Z`],
  redo: isMac() ? ["Cmd+Shift+Z"] : ["Ctrl+Y", "Ctrl+Shift+Z"],
};

/**
 * The shortcuts shown in the cheatsheet, excluding the global toggle hotkey
 * (that one is dynamic — ShortcutsPanel prepends it from the settings).
 */
export function appShortcuts(): ShortcutDef[] {
  const rows: ShortcutDef[] = [
    { id: "new-note", keys: SHORTCUT_KEYS["new-note"], label: t.sNewNote, group: "app" },
    { id: "next-note", keys: SHORTCUT_KEYS["next-note"], label: t.sNextNote, group: "app" },
    { id: "prev-note", keys: SHORTCUT_KEYS["prev-note"], label: t.sPrevNote, group: "app" },
    { id: "open-settings", keys: SHORTCUT_KEYS["open-settings"], label: t.sOpenSettings, group: "app" },
    { id: "cycle-theme", keys: SHORTCUT_KEYS["cycle-theme"], label: t.sCycleTheme, group: "app" },
    { id: "show-shortcuts", keys: SHORTCUT_KEYS["show-shortcuts"], label: t.sShowShortcuts, group: "app" },
    { id: "bold", keys: SHORTCUT_KEYS["bold"], label: t.sBold, group: "formatting" },
    { id: "italic", keys: SHORTCUT_KEYS["italic"], label: t.sItalic, group: "formatting" },
    { id: "strike", keys: SHORTCUT_KEYS["strike"], label: t.sStrike, group: "formatting" },
    { id: "inline-code", keys: SHORTCUT_KEYS["inline-code"], label: t.sInlineCode, group: "formatting" },
    { id: "check-item", keys: SHORTCUT_KEYS["check-item"], label: t.sCheckItem, group: "formatting" },
    { id: "undo", keys: SHORTCUT_KEYS["undo"], label: t.sUndo, group: "formatting" },
    { id: "redo", keys: SHORTCUT_KEYS["redo"], label: t.sRedo, group: "formatting" },
  ];
  return rows;
}
