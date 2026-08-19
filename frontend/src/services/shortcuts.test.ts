import { describe, expect, it } from "vitest";
import { APP_SHORTCUTS, kbdParts, SHORTCUT_GROUPS } from "./shortcuts";

describe("kbdParts", () => {
  it("splits an accelerator into key parts", () => {
    expect(kbdParts("Ctrl+Shift+Tab")).toEqual(["Ctrl", "Shift", "Tab"]);
    expect(kbdParts("Alt+Shift+S")).toEqual(["Alt", "Shift", "S"]);
  });

  it("handles empty / malformed combos without crashing", () => {
    expect(kbdParts("")).toEqual([]);
    expect(kbdParts("++")).toEqual([]);
  });
});

describe("APP_SHORTCUTS registry", () => {
  it("every shortcut has an id, at least one combo, a label and a valid group", () => {
    const groups = new Set(SHORTCUT_GROUPS.map((g) => g.id));
    for (const s of APP_SHORTCUTS) {
      expect(s.id).toBeTruthy();
      expect(s.keys.length).toBeGreaterThan(0);
      for (const k of s.keys) {
        // Accelerator shape: modifier+key, e.g. "Ctrl+,", "Ctrl+Shift+T".
        expect(k).toBeTruthy();
        expect(k).not.toMatch(/^\+/);
        expect(k).not.toMatch(/\+$/);
        expect(k).not.toContain(" ");
      }
      expect(s.label).toBeTruthy();
      expect(groups.has(s.group)).toBe(true);
    }
  });

  it("app-level shortcuts use Ctrl (not Alt/Meta) so they never collide with OS combos", () => {
    for (const s of APP_SHORTCUTS.filter((x) => x.group === "app")) {
      for (const k of s.keys) {
        expect(k.startsWith("Ctrl+")).toBe(true);
      }
    }
  });

  it("shortcut ids are unique", () => {
    const ids = APP_SHORTCUTS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});
