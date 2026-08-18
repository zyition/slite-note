/**
 * hotkey.ts — mapping a keyboard event to a Wails accelerator string
 * ("Alt+Shift+S", "Ctrl+K", …) and formatting one for display.
 *
 * The allowed key set mirrors what Wails' parseAccelerator accepts (single
 * printable chars plus the named keys) while excluding fragile combos like a
 * bare modifier or the Windows key alone.
 */

const NAMED_KEYS: Record<string, string> = {
  " ": "Space",
  Enter: "Enter",
  Tab: "Tab",
  ArrowUp: "Up",
  ArrowDown: "Down",
  ArrowLeft: "Left",
  ArrowRight: "Right",
  Home: "Home",
  End: "End",
};

function modifierOf(e: KeyboardEvent): string | null {
  const parts: string[] = [];
  if (e.ctrlKey) parts.push("Ctrl");
  if (e.altKey) parts.push("Alt");
  if (e.shiftKey) parts.push("Shift");
  if (e.metaKey) parts.push("Super");
  return parts.length ? parts.join("+") : null;
}

/**
 * Formats a keydown event as an accelerator, or null when the event does not
 * carry a usable main key (e.g. a bare modifier press). Escape is signalled
 * separately by the caller (returns null).
 */
export function formatCombo(e: KeyboardEvent): string | null {
  if (e.key === "Escape") return null; // caller uses this to cancel recording
  const mod = modifierOf(e) ?? "";
  if (mod === "") return null; // require at least one modifier

  const key = e.key;
  let main: string | null = null;

  if (key.length === 1 && /^[a-zA-Z0-9]$/.test(key)) {
    main = key.toUpperCase();
  } else if (/^F([1-9]|1[0-2])$/.test(key)) {
    main = key;
  } else if (key in NAMED_KEYS) {
    main = NAMED_KEYS[key];
  }
  if (!main) return null;

  // A lone modifier (Ctrl/Alt/Shift/Meta by itself) has no main key.
  if (main === "Space" && !mod) return null;
  return `${mod}+${main}`;
}

/** Human-readable display of an accelerator (already +-joined). */
export function displayCombo(combo: string): string {
  return combo || "—";
}
