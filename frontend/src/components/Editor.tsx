import { useCreateBlockNote } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/mantine";
import type { Block, PartialBlock } from "@blocknote/core";
import type { Note } from "../types/note";

interface EditorProps {
  note: Note;
  blocknoteTheme: "light" | "dark";
  /** Fired on every document change; App debounces + persists. */
  onChange: (blocks: Block[]) => void;
}

/**
 * Editor — BlockNote core wrapper. Remounts per note (App keys it by note id)
 * so switching notes reloads `initialContent` cleanly.
 */
export function Editor({ note, blocknoteTheme, onChange }: EditorProps) {
  const editor = useCreateBlockNote({
    initialContent: (note.blocks?.length ? note.blocks : undefined) as
      | PartialBlock[]
      | undefined,
  });

  return (
    <BlockNoteView
      editor={editor}
      theme={blocknoteTheme}
      onChange={() => onChange(editor.document)}
      data-testid="slite-editor"
    />
  );
}
