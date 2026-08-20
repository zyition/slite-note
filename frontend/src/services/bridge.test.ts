import { beforeEach, describe, expect, it, vi } from "vitest";

// In node there is no Wails runtime: force the bindings' Ping probe to fail so
// bridge.ts takes the localStorage fallback path (the browser-only dev mode).
vi.mock("@wailsio/runtime", () => ({
  Events: { On: vi.fn() },
  Window: {
    Hide: vi.fn(),
    SetAlwaysOnTop: vi.fn(),
    SetBackgroundColour: vi.fn(),
  },
}));

vi.mock("../../bindings/github.com/zyition/slite-note", () => ({
  Store: {
    Ping: vi.fn().mockRejectedValue(new Error("not in native runtime")),
  },
}));

import {
  loadNotes,
  loadSettings,
  saveNote,
  deleteNote,
  saveSettings,
} from "./bridge";
import { makeSettings } from "../types/note";

function makeStorage() {
  const m = new Map<string, string>();
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
    removeItem: (k: string) => void m.delete(k),
    clear: () => void m.clear(),
    key: (i: number) => [...m.keys()][i] ?? null,
    get length() {
      return m.size;
    },
  };
}

beforeEach(() => {
  vi.stubGlobal("localStorage", makeStorage());
});

describe("bridge localStorage fallback", () => {
  it("loadNotes returns an empty list when nothing is stored", async () => {
    expect(await loadNotes()).toEqual([]);
  });

  it("saveNote + loadNotes round-trip", async () => {
    const note = { id: "a", title: "", blocks: [], createdAt: 1, updatedAt: 2 };
    await saveNote(note);
    expect(await loadNotes()).toEqual([note]);
  });

  it("saveNote upserts and deleteNote removes", async () => {
    const a = { id: "a", title: "", blocks: [], createdAt: 1, updatedAt: 1 };
    await saveNote(a);
    await saveNote({ ...a, title: "edited", updatedAt: 2 });
    expect(await loadNotes()).toEqual([{ ...a, title: "edited", updatedAt: 2 }]);
    await deleteNote("a");
    expect(await loadNotes()).toEqual([]);
  });

  it("loadNotes tolerates corrupt JSON", async () => {
    localStorage.setItem("slite:notes", "{broken");
    expect(await loadNotes()).toEqual([]);
  });

  it("loadSettings falls back to defaults", async () => {
    expect(await loadSettings()).toEqual(makeSettings());
  });

  it("saveSettings + loadSettings round-trip", async () => {
    await saveSettings({ ...makeSettings(), theme: "dark", alwaysOnTop: true });
    const got = await loadSettings();
    expect(got.theme).toBe("dark");
    expect(got.alwaysOnTop).toBe(true);
  });

  it("loadSettings tolerates corrupt JSON", async () => {
    localStorage.setItem("slite:settings", "not json");
    expect(await loadSettings()).toEqual(makeSettings());
  });

  it("uses the slite: namespaced keys", async () => {
    await saveNote({ id: "x", title: "", blocks: [], createdAt: 1, updatedAt: 1 });
    await saveSettings(makeSettings({ theme: "gray" }));
    expect(localStorage.getItem("slite:notes")).toContain('"id":"x"');
    expect(localStorage.getItem("slite:settings")).toContain('"theme":"gray"');
  });
});
