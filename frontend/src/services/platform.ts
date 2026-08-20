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
