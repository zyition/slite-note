import type { ThemeName } from "../types/note";

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
 * preference: dark OS → dark theme, light OS → gray theme.
 */
export function resolveTheme(choice: ThemeName): Exclude<ThemeName, "system"> {
  if (choice === "system") return systemPrefersDark() ? "dark" : "gray";
  return choice;
}

/** Subscribe to OS dark-mode changes; returns an unsubscribe function. */
export function onSystemThemeChange(cb: () => void): () => void {
  if (!mq) return () => {};
  const h = () => cb();
  mq.addEventListener("change", h);
  return () => mq.removeEventListener("change", h);
}

/** Short display name of a concrete theme (theme-picker rows). */
export function themeName(theme: Exclude<ThemeName, "system">): string {
  switch (theme) {
    case "dark":
      return "Dark";
    case "gray":
      return "Gray";
    case "yellow":
      return "Yellow";
  }
}

/** CSS colour of a theme's swatch (matches its --bg). */
export function themeSwatch(theme: Exclude<ThemeName, "system">): string {
  const [r, g, b] = THEMES[theme].rgb;
  return `rgb(${r}, ${g}, ${b})`;
}

/** Full label for the picker trigger tooltip, e.g. "System (Dark)". */
export function themeLabel(
  choice: ThemeName,
  applied: Exclude<ThemeName, "system">,
): string {
  return choice === "system" ? `System (${themeName(applied)})` : themeName(applied);
}
