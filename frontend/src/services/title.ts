import type { Block } from "@blocknote/core";

const MAX_TITLE_LEN = 28;

/**
 * deriveTitle returns the display title of a note: the first non-empty line of
 * text from its blocks (per domain model: title defaults to first line; a
 * manual override field exists but has no UI in the MVP).
 */
export function deriveTitle(blocks: unknown[] | null | undefined): string {
  if (!Array.isArray(blocks)) return "";
  for (const block of blocks) {
    const text = blockText(block);
    const trimmed = text.trim();
    if (trimmed) {
      return trimmed.length > MAX_TITLE_LEN
        ? trimmed.slice(0, MAX_TITLE_LEN).trimEnd() + "…"
        : trimmed;
    }
  }
  return "";
}

function blockText(block: any): string {
  if (!block || typeof block !== "object") return "";
  const content = block.content;
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    const joined = content
      .map((item: any) => {
        if (typeof item === "string") return item;
        if (item && typeof item === "object") {
          if (typeof item.text === "string") return item.text;
          if (Array.isArray(item.content)) return blockText(item);
        }
        return "";
      })
      .join("");
    // An empty content array (e.g. an empty list item) must not short-circuit
    // the children recursion below — the container may still hold text.
    if (joined.trim()) return joined;
  }
  // Recurse into children in case the first block is an empty container.
  if (Array.isArray(block.children)) {
    for (const child of block.children) {
      const text = blockText(child);
      if (text.trim()) return text;
    }
  }
  return "";
}

export type { Block };
