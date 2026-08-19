import { useCallback, useEffect, useRef } from "react";
import { useCreateBlockNote, SideMenuController } from "@blocknote/react";
import { BlockNoteView } from "@blocknote/mantine";
import { BlockNoteSchema, defaultBlockSpecs } from "@blocknote/core";
import type { Block, PartialBlock } from "@blocknote/core";
import { BlockSideMenu } from "./BlockSideMenu";
import { onShow } from "../services/bridge";
import type { Note } from "../types/note";

/**
 * Sticky notes have no attachment pipeline yet, so the media blocks
 * (image / video / audio / file) are excluded from the schema. Every UI
 * entry point (slash menu, block-type select, …) is generated from the
 * schema, so they disappear everywhere at once — no per-component CSS or
 * menu filtering needed.
 */
const sliteSchema = BlockNoteSchema.create({
  blockSpecs: {
    paragraph: defaultBlockSpecs.paragraph,
    heading: defaultBlockSpecs.heading,
    bulletListItem: defaultBlockSpecs.bulletListItem,
    numberedListItem: defaultBlockSpecs.numberedListItem,
    checkListItem: defaultBlockSpecs.checkListItem,
    toggleListItem: defaultBlockSpecs.toggleListItem,
    quote: defaultBlockSpecs.quote,
    codeBlock: defaultBlockSpecs.codeBlock,
    table: defaultBlockSpecs.table,
    divider: defaultBlockSpecs.divider,
  },
});

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
 * Caret continuity: we remember the caret position when the window loses
 * focus (alt-tab away, or hiding) and restore it when the window comes back
 * (refocus or app:show). When nothing was remembered we land at the end of
 * the document — the note cannot change while the window is away, so the
 * stored position stays valid.
 */
export function Editor({ note, blocknoteTheme, onChange }: EditorProps) {
  const editor = useCreateBlockNote({
    schema: sliteSchema,
    initialContent: (note.blocks?.length ? note.blocks : undefined) as
      | PartialBlock[]
      | undefined,
    // Trailing fake paragraph: fills the space below the content and turns
    // into a real block when clicked (see index.css for the full-height
    // styling).
    trailingBlock: true,
  });

  // Absolute ProseMirror position of the caret when the window lost focus,
  // restored on refocus. null → no memory (fall back to the document end).
  const caretRef = useRef<number | null>(null);

  // Restore the remembered caret, or fall back to the end of the note.
  const placeCaret = useCallback(() => {
    const blocks = editor.document;
    if (!blocks.length) return;
    const pm = editor._tiptapEditor;
    const saved = caretRef.current;
    if (saved !== null && saved <= pm.state.doc.content.size) {
      pm.commands.setTextSelection(saved);
      editor.focus();
    } else {
      editor.focus();
      editor.setTextCursorPosition(blocks[blocks.length - 1], "end");
    }
  }, [editor]);

  // Window summoned (hotkey / tray): restore the caret from before it was
  // hidden, or fall back to the end — same as a plain refocus.
  useEffect(() => {
    return onShow(placeCaret);
  }, [placeCaret]);

  // Alt-tab away/back: remember the caret on blur, restore it on refocus.
  // Hiding the window also blurs it, so the summon path above reuses the
  // same memory.
  useEffect(() => {
    const pm = editor._tiptapEditor;
    const onWindowBlur = () => {
      caretRef.current = pm.state.selection.from;
    };
    window.addEventListener("blur", onWindowBlur);
    window.addEventListener("focus", placeCaret);
    return () => {
      window.removeEventListener("blur", onWindowBlur);
      window.removeEventListener("focus", placeCaret);
    };
  }, [editor, placeCaret]);

  // Ctrl+Enter toggles the checklist item under the caret (Notion/Typora
  // convention). BlockNote has no built-in binding and the slash menu does
  // not show this, so we handle it ourselves. Capture phase: it must work
  // even when ProseMirror has focus.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!e.ctrlKey || e.altKey || e.metaKey || e.key !== "Enter") return;
      const { block } = editor.getTextCursorPosition();
      if (block.type !== "checkListItem") return;
      e.preventDefault();
      e.stopPropagation();
      editor.updateBlock(block.id, {
        props: { checked: !(block.props as { checked?: boolean }).checked },
      });
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [editor]);

  return (
    <div
      className="h-full"
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
