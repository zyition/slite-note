import type { Note } from "../types/note";
import { deriveTitle } from "./title";

interface TitleEntry {
  blocks: Note["blocks"];
  title: string;
  text: string;
}

type DeriveFn = (blocks: Note["blocks"]) => string;

/**
 * Per-note display-title cache.
 *
 * A note's title is either an explicit manual override (`note.title`) or
 * derived from the first non-empty line of its blocks. React state updates
 * are immutable, so a note's `blocks` / `title` reference moving is an exact
 * "this note changed" signal: an entry stays valid until its note's
 * references move. Repeated lookups are O(1); re-derivation is O(changed
 * note only) — never O(all notes).
 *
 * `derive` is injectable so tests can observe when derivation actually runs.
 */
export class TitleCache {
  private entries = new Map<string, TitleEntry>();
  private readonly derive: DeriveFn;

  constructor(derive: DeriveFn = (blocks) => deriveTitle(blocks)) {
    this.derive = derive;
  }

  titleFor(note: Note): string {
    const cached = this.entries.get(note.id);
    if (cached && cached.blocks === note.blocks && cached.title === note.title) {
      return cached.text;
    }
    const text = note.title.trim() ? note.title : this.derive(note.blocks);
    this.entries.set(note.id, { blocks: note.blocks, title: note.title, text });
    return text;
  }

  /** Forget a note that was deleted from the list so it cannot linger. */
  delete(id: string): void {
    this.entries.delete(id);
  }
}
