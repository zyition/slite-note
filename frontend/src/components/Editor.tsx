import { useCreateBlockNote, SideMenuController } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/mantine";
import type { Block, PartialBlock } from "@blocknote/core";
import { BlockSideMenu } from "./BlockSideMenu";
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
 *
 * Click-to-focus: BlockNote only renders as tall as its content, so clicking
 * the empty area below the last block does nothing by default. We listen for
 * clicks that land *outside* any block (`.bn-block`) and move the cursor to
 * the end of the document instead — the classic "tap below to keep typing"
 * behaviour.
 */
export function Editor({ note, blocknoteTheme, onChange }: EditorProps) {
  const editor = useCreateBlockNote({
    initialContent: (note.blocks?.length ? note.blocks : undefined) as
      | PartialBlock[]
      | undefined,
  });

  const handleMouseDown = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    // Clicks inside a block (text, list items, …) keep the default behaviour.
    if (target.closest(".bn-block")) return;
    // Clicks on the empty area below the content jump to the last block end.
    const blocks = editor.document;
    if (!blocks.length) return;
    editor.focus();
    editor.setTextCursorPosition(blocks[blocks.length - 1], "end");
  };

  return (
    <div
      className="h-full"
      onMouseDown={handleMouseDown}
      // Block the editor's default context menu for now; we will build a
      // custom one later when needed.
      onContextMenu={(e) => e.preventDefault()}
    >
      <BlockNoteView
        editor={editor}
        theme={blocknoteTheme}
        onChange={() => onChange(editor.document)}
        sideMenu={false}
        data-testid="slite-editor"
      >
        <SideMenuController sideMenu={BlockSideMenu} />
      </BlockNoteView>
    </div>
  );
}
