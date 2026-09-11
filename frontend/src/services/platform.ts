/**
 * platform.ts — lightweight platform detection shared by the shortcut layers.
 * Kept dependency-free so hotkey.ts / shortcuts.ts can import it in tests.
 */

/** Whether the app runs on macOS (drives Mod-mapped shortcuts + ⌘ symbols). */
export function isMac(): boolean {
  if (typeof navigator === "undefined") return false;
  // navigator.platform is deprecated but still accurate in WebKit/Chromium;
  // userAgent is the fallback for privacy-tightened builds.
  return (
    /Mac|iPhone|iPad|iPod/.test(navigator.platform) ||
    /Macintosh/.test(navigator.userAgent)
  );
}

/** Whether the event carries the platform's primary modifier (Cmd on macOS,
 * Ctrl elsewhere) and not the *other* platform's one — so a Cmd shortcut does
 * not fire on Ctrl and vice versa. Shared by every manual shortcut handler.
 * (altKey/shiftKey are left to the caller: some shortcuts want them.) */
export function hasPrimaryModifier(event: {
  metaKey: boolean;
  ctrlKey: boolean;
}): boolean {
  return isMac()
    ? event.metaKey && !event.ctrlKey
    : event.ctrlKey && !event.metaKey;
}

/** The modifier symbol shown next to shortcut labels in menus. */
export const SHORTCUT_MODIFIER = isMac() ? "⌘" : "Ctrl+";
