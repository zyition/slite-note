import { describe, expect, it, vi } from "vitest";
import { TitleCache } from "./titleCache";
import type { Note } from "../types/note";

const makeNote = (id: string, title: string, blocks: Note["blocks"]): Note => ({
  id,
  title,
  blocks,
  createdAt: 1,
  updatedAt: 1,
});

const textBlock = (text: string) => ({
  type: "paragraph",
  content: [{ type: "text", text, styles: {} }],
});

describe("TitleCache", () => {
  it("derives the first non-empty line when title is empty", () => {
    const cache = new TitleCache();
    expect(cache.titleFor(makeNote("a", "", [textBlock("Hello")]))).toBe("Hello");
  });

  it("prefers a manual title override without deriving", () => {
    const derive = vi.fn(() => "derived");
    const cache = new TitleCache(derive);
    const note = makeNote("a", "Manual", [textBlock("Hello")]);
    expect(cache.titleFor(note)).toBe("Manual");
    expect(derive).not.toHaveBeenCalled();
  });

  it("returns the manual title as-is, including surrounding whitespace", () => {
    const cache = new TitleCache();
    expect(cache.titleFor(makeNote("a", "  spaced  ", []))).toBe("  spaced  ");
  });

  it("falls back to deriving when the manual title is blank", () => {
    const cache = new TitleCache();
    expect(cache.titleFor(makeNote("a", "   ", [textBlock("x")]))).toBe("x");
  });

  it("caches by reference: repeated lookups on the same note are O(1)", () => {
    const derive = vi.fn(() => "derived");
    const cache = new TitleCache(derive);
    const note = makeNote("a", "", [textBlock("x")]);
    cache.titleFor(note);
    cache.titleFor(note);
    cache.titleFor(note);
    expect(derive).toHaveBeenCalledTimes(1);
  });

  it("re-derives only when the blocks reference moves", () => {
    const derive = vi.fn(() => "derived");
    const cache = new TitleCache(derive);
    const first = makeNote("a", "", [textBlock("one")]);
    cache.titleFor(first);
    // Same id, brand-new blocks object — even with identical content.
    const edited = makeNote("a", "", [textBlock("one")]);
    cache.titleFor(edited);
    expect(derive).toHaveBeenCalledTimes(2);
  });

  it("recomputes on rename but keeps manual priority (no derive)", () => {
    const derive = vi.fn(() => "derived");
    const cache = new TitleCache(derive);
    const note = makeNote("a", "", [textBlock("x")]);
    expect(cache.titleFor(note)).toBe("derived");
    const renamed = makeNote("a", "Renamed", note.blocks);
    expect(cache.titleFor(renamed)).toBe("Renamed");
    expect(derive).toHaveBeenCalledTimes(1);
  });

  it("re-derives when a manual title is cleared (restore derived)", () => {
    const derive = vi.fn((blocks: Note["blocks"]) =>
      Array.isArray(blocks) && blocks.length > 0 ? "line" : "",
    );
    const cache = new TitleCache(derive);
    const note = makeNote("a", "Manual", [textBlock("x")]);
    expect(cache.titleFor(note)).toBe("Manual");
    const cleared = makeNote("a", "", note.blocks);
    expect(cache.titleFor(cleared)).toBe("line");
    expect(derive).toHaveBeenCalledTimes(1);
  });

  it("keeps notes independent: editing one never recomputes another", () => {
    const derive = vi.fn(() => "derived");
    const cache = new TitleCache(derive);
    const a1 = makeNote("a", "", [textBlock("A1")]);
    const b = makeNote("b", "", [textBlock("B")]);
    cache.titleFor(a1);
    cache.titleFor(b);
    // Edit note a only — b's entry must survive untouched.
    const a2 = makeNote("a", "", [textBlock("A2")]);
    cache.titleFor(a2);
    expect(cache.titleFor(b)).toBe("derived");
    expect(derive).toHaveBeenCalledTimes(3); // a1, b, a2 — b never recomputed
  });

  it("forgets a note's entry on delete()", () => {
    const derive = vi.fn(() => "derived");
    const cache = new TitleCache(derive);
    const note = makeNote("a", "", [textBlock("x")]);
    cache.titleFor(note);
    cache.delete("a");
    cache.titleFor(note);
    expect(derive).toHaveBeenCalledTimes(2);
  });

  it("handles null blocks safely", () => {
    const cache = new TitleCache();
    expect(cache.titleFor(makeNote("a", "", null))).toBe("");
  });
});
