import { describe, expect, it } from "vitest";
import {
  resolveTheme,
  themeLabel,
  themeName,
  themeSwatch,
  THEMES,
} from "./theme";

describe("resolveTheme", () => {
  it("passes concrete themes through", () => {
    expect(resolveTheme("yellow")).toBe("yellow");
    expect(resolveTheme("gray")).toBe("gray");
    expect(resolveTheme("dark")).toBe("dark");
  });

  it("resolves system to gray when the OS is light (no matchMedia in node)", () => {
    // window is undefined under vitest node env → systemPrefersDark() = false.
    expect(resolveTheme("system")).toBe("gray");
  });
});

describe("theme helpers", () => {
  it("maps concrete themes to display names", () => {
    expect(themeName("dark")).toBe("Dark");
    expect(themeName("gray")).toBe("Gray");
    expect(themeName("yellow")).toBe("Yellow");
  });

  it("themeSwatch mirrors the theme rgb", () => {
    expect(themeSwatch("yellow")).toBe(`rgb(${THEMES.yellow.rgb.join(", ")})`);
  });

  it("themeLabel shows the resolved theme for system", () => {
    expect(themeLabel("system", "dark")).toBe("System (Dark)");
    expect(themeLabel("yellow", "yellow")).toBe("Yellow");
  });

  it("every theme has a valid rgb triple and blocknote mode", () => {
    for (const def of Object.values(THEMES)) {
      expect(def.rgb).toHaveLength(3);
      for (const c of def.rgb) {
        expect(c).toBeGreaterThanOrEqual(0);
        expect(c).toBeLessThanOrEqual(255);
      }
      expect(["light", "dark"]).toContain(def.blocknote);
    }
  });
});
