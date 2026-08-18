import type { ThemeName } from "../types/note";

export interface ThemeDef {
  /** CSS variables applied via [data-theme] (see index.css). */
  name: ThemeName;
  /** Window background colour (matches --bg) for the WebView2 shell. */
  rgb: [number, number, number];
  /** BlockNote editor theme: dark content on light themes and vice versa. */
  blocknote: "light" | "dark";
}

export const THEMES: Record<ThemeName, ThemeDef> = {
  yellow: { name: "yellow", rgb: [255, 248, 220], blocknote: "light" },
  gray: { name: "gray", rgb: [244, 244, 242], blocknote: "light" },
  dark: { name: "dark", rgb: [31, 31, 31], blocknote: "dark" },
};

export function cycleTheme(current: ThemeName): ThemeName {
  const order: ThemeName[] = ["yellow", "gray", "dark"];
  const i = order.indexOf(current);
  return order[(i + 1) % order.length];
}
