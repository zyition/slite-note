import { describe, expect, it } from "vitest";
import { displayCombo, formatCombo } from "./hotkey";

const key = (partial: Partial<KeyboardEvent>): KeyboardEvent =>
  ({
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    metaKey: false,
    key: "",
    ...partial,
  }) as KeyboardEvent;

describe("formatCombo", () => {
  it("formats modifier + letter combos", () => {
    expect(formatCombo(key({ ctrlKey: true, key: "k" }))).toBe("Ctrl+K");
    expect(
      formatCombo(key({ altKey: true, shiftKey: true, key: "s" })),
    ).toBe("Alt+Shift+S");
    expect(formatCombo(key({ ctrlKey: true, shiftKey: true, key: "t" }))).toBe(
      "Ctrl+Shift+T",
    );
  });

  it("uppercases single letters and keeps digits", () => {
    expect(formatCombo(key({ ctrlKey: true, key: "K" }))).toBe("Ctrl+K");
    expect(formatCombo(key({ ctrlKey: true, key: "5" }))).toBe("Ctrl+5");
  });

  it("formats function keys", () => {
    expect(formatCombo(key({ ctrlKey: true, key: "F5" }))).toBe("Ctrl+F5");
    expect(formatCombo(key({ ctrlKey: true, key: "F12" }))).toBe("Ctrl+F12");
  });

  it("formats named keys (Wails accelerator names)", () => {
    // Browser key events report "ArrowUp"; Wails' parseAccelerator only
    // accepts "up"/"down"/"left"/"right" (see hotkey.ts NAMED_KEYS).
    expect(formatCombo(key({ ctrlKey: true, key: "ArrowUp" }))).toBe(
      "Ctrl+Up",
    );
    expect(formatCombo(key({ ctrlKey: true, key: "Enter" }))).toBe("Ctrl+Enter");
    expect(formatCombo(key({ ctrlKey: true, key: "Tab" }))).toBe("Ctrl+Tab");
    expect(formatCombo(key({ ctrlKey: true, key: " " }))).toBe("Ctrl+Space");
  });

  it("returns null when no modifier is held", () => {
    expect(formatCombo(key({ key: "k" }))).toBe(null);
    expect(formatCombo(key({ key: "F5" }))).toBe(null);
  });

  it("returns null for Escape (caller cancels recording)", () => {
    expect(formatCombo(key({ ctrlKey: true, key: "Escape" }))).toBe(null);
  });

  it("returns null for bare modifier presses and unsupported keys", () => {
    expect(formatCombo(key({ ctrlKey: true }))).toBe(null); // Ctrl alone
    expect(formatCombo(key({ ctrlKey: true, key: "CapsLock" }))).toBe(null);
    expect(formatCombo(key({ ctrlKey: true, key: "ContextMenu" }))).toBe(null);
  });

  it("returns null for bare Space (would be a useless toggle)", () => {
    expect(formatCombo(key({ key: " " }))).toBe(null);
  });
});

describe("displayCombo", () => {
  it("shows the combo or an em dash when empty", () => {
    expect(displayCombo("Alt+Shift+S")).toBe("Alt+Shift+S");
    expect(displayCombo("")).toBe("—");
    expect(displayCombo(undefined as unknown as string)).toBe("—");
  });
});
