import { describe, expect, it } from "vitest";
import { deriveTitle } from "./title";

const textBlock = (text: string) => ({
  type: "paragraph",
  content: [{ type: "text", text, styles: {} }],
});

describe("deriveTitle", () => {
  it("returns the first non-empty line of text", () => {
    expect(deriveTitle([textBlock("Hello world")])).toBe("Hello world");
  });

  it("returns empty string for empty / missing blocks", () => {
    expect(deriveTitle([])).toBe("");
    expect(deriveTitle(null)).toBe("");
    expect(deriveTitle(undefined)).toBe("");
    expect(deriveTitle([{ type: "paragraph", content: [] }])).toBe("");
  });

  it("skips empty leading blocks and takes the first with text", () => {
    const blocks = [
      { type: "paragraph", content: [] },
      { type: "heading", content: "Second" },
    ];
    expect(deriveTitle(blocks)).toBe("Second");
  });

  it("trims surrounding whitespace", () => {
    expect(deriveTitle([textBlock("  padded  ")])).toBe("padded");
  });

  it("truncates long titles to 28 chars with an ellipsis", () => {
    const long = "a".repeat(40);
    expect(deriveTitle([textBlock(long)])).toBe("a".repeat(28) + "…");
  });

  it("handles plain-string block content", () => {
    expect(deriveTitle([{ type: "heading", content: "Title" }])).toBe("Title");
  });

  it("recurses into children of an empty container block", () => {
    const blocks = [
      { type: "listItem", content: [], children: [textBlock("Child text")] },
    ];
    expect(deriveTitle(blocks)).toBe("Child text");
  });

  it("treats non-array blocks as empty", () => {
    expect(deriveTitle("not an array" as unknown as null)).toBe("");
  });
});
