import { describe, expect, it, vi } from "vitest";

// Default module load: non-mac (jsdom reports an empty platform), which
// exercises the Windows/Linux Ctrl-mapped path.
import { appShortcuts, SHORTCUT_GROUPS } from "./shortcuts";

describe("appShortcuts (Windows/Linux path)", () => {
  it("every shortcut has an id, at least one combo, a label and a valid group", () => {
    const groups = new Set(SHORTCUT_GROUPS.map((g) => g.id));
    for (const s of appShortcuts()) {
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
    for (const s of appShortcuts().filter((x) => x.group === "app")) {
      for (const k of s.keys) {
        expect(k.startsWith("Ctrl+")).toBe(true);
      }
    }
  });

  it("maps note switching to Ctrl+Tab / Ctrl+Shift+Tab on Windows", () => {
    const rows = appShortcuts();
    expect(rows.find((r) => r.id === "next-note")?.keys).toEqual(["Ctrl+Tab"]);
    expect(rows.find((r) => r.id === "prev-note")?.keys).toEqual([
      "Ctrl+Shift+Tab",
    ]);
  });

  it("shortcut ids are unique", () => {
    const ids = appShortcuts().map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe("appShortcuts (macOS path)", () => {
  it("Mod-maps app shortcuts to Cmd and overrides note switching", async () => {
    vi.resetModules();
    vi.doMock("./platform", () => ({ isMac: () => true }));
    const mac = await import("./shortcuts");
    const rows = mac.appShortcuts();

    expect(rows.find((r) => r.id === "new-note")?.keys).toEqual(["Cmd+N"]);
    expect(rows.find((r) => r.id === "open-settings")?.keys).toEqual(["Cmd+,"]);
    // Ctrl+Tab is the system keyboard-navigation combo on macOS, Cmd+Tab the
    // app switcher: note switching moves to the browser tab idiom.
    expect(rows.find((r) => r.id === "next-note")?.keys).toEqual([
      "Cmd+Shift+]",
    ]);
    expect(rows.find((r) => r.id === "prev-note")?.keys).toEqual([
      "Cmd+Shift+[",
    ]);
    expect(rows.find((r) => r.id === "check-item")?.keys).toEqual([
      "Cmd+Enter",
    ]);
    // Redo follows the mac convention.
    expect(rows.find((r) => r.id === "redo")?.keys).toEqual(["Cmd+Shift+Z"]);

    vi.doUnmock("./platform");
  });
});
