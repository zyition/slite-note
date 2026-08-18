import type { ThemeName } from "../types/note";
import { THEME_NAMES } from "../types/note";

export interface ThemeDef {
  /** CSS variables applied via [data-theme] (see index.css). */
  name: ThemeName;
  /** Window background colour (matches --bg) for the WebView2 shell. */
  rgb: [number, number, number];
  /** BlockNote editor theme: dark content on light themes and vice versa. */
  blocknote: "light" | "dark";
}

/** The three concrete themes. "system" resolves to one of these. */
export const THEMES: Record<Exclude<ThemeName, "system">, ThemeDef> = {
  yellow: { name: "yellow", rgb: [255, 248, 220], blocknote: "light" },
  gray: { name: "gray", rgb: [244, 244, 242], blocknote: "light" },
  dark: { name: "dark", rgb: [31, 31, 31], blocknote: "dark" },
};

const mq = typeof window !== "undefined" ? window.matchMedia("(prefers-color-scheme: dark)") : null;

/** Whether the OS is currently in dark mode (browser fallback: false). */
export function systemPrefersDark(): boolean {
  return mq?.matches ?? false;
}

/**
 * Resolve a theme choice to a concrete theme name. "system" follows the OS
 * preference: dark OS → dark theme, light OS → the default yellow sticky.
 */
export function resolveTheme(choice: ThemeName): Exclude<ThemeName, "system"> {
  if (choice === "system") return systemPrefersDark() ? "dark" : "yellow";
  return choice;
}

/** Subscribe to OS dark-mode changes; returns an unsubscribe function. */
export function onSystemThemeChange(cb: () => void): () => void {
  if (!mq) return () => {};
  const h = () => cb();
  mq.addEventListener("change", h);
  return () => mq.removeEventListener("change", h);
}

/** Cycle through yellow → gray → dark → system → yellow. */
export function cycleTheme(current: ThemeName): ThemeName {
  const i = THEME_NAMES.indexOf(current);
  return THEME_NAMES[(i + 1) % THEME_NAMES.length];
}
