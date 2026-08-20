import { describe, expect, it } from "vitest";
import { makeNote, makeSettings, THEME_NAMES, uuid } from "./note";

describe("makeNote", () => {
  it("creates a note with a uuid and timestamps", () => {
    const before = Date.now();
    const n = makeNote();
    const after = Date.now();
    expect(n.id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
    expect(n.title).toBe("");
    expect(n.blocks).toEqual([]);
    expect(n.createdAt).toBeGreaterThanOrEqual(before);
    expect(n.createdAt).toBeLessThanOrEqual(after);
    expect(n.updatedAt).toBe(n.createdAt);
  });

  it("generates unique ids", () => {
    expect(makeNote().id).not.toBe(makeNote().id);
  });
});

describe("uuid", () => {
  it("generates v4-shaped uuids", () => {
    const id = uuid();
    expect(id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
  });
});

describe("makeSettings", () => {
  it("fills sensible defaults", () => {
    const s = makeSettings();
    expect(s).toEqual({
      theme: "system",
      alwaysOnTop: false,
      hotkey: "Alt+Shift+S",
      launchAtStartup: false,
      opacity: 1,
      windowX: 0,
      windowY: 0,
      windowWidth: 0,
      windowHeight: 0,
    });
  });

  it("lets callers override individual fields", () => {
    const s = makeSettings({ theme: "dark", alwaysOnTop: true });
    expect(s.theme).toBe("dark");
    expect(s.alwaysOnTop).toBe(true);
    expect(s.hotkey).toBe("Alt+Shift+S"); // untouched
  });
});

describe("THEME_NAMES", () => {
  it("is the ordered list of user-selectable themes", () => {
    expect(THEME_NAMES).toEqual(["system", "dark", "gray", "yellow"]);
  });
});
