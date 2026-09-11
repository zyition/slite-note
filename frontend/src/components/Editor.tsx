import { useCallback, useEffect, useRef, useState } from "react";
import type {
  ClipboardEvent,
  DragEvent as ReactDragEvent,
  MouseEvent as ReactMouseEvent,
} from "react";
import {
  useCreateBlockNote,
  LinkToolbarController,
  SideMenuController,
  SuggestionMenuController,
  getDefaultReactSlashMenuItems,
} from "@blocknote/react";
import { BlockNoteView } from "@blocknote/mantine";
import { BlockNoteSchema, defaultBlockSpecs, markdownToBlocks } from "@blocknote/core";
import { filterSuggestionItems } from "@blocknote/core/extensions";
import { zh } from "@blocknote/core/locales";
import type { Block, PartialBlock } from "@blocknote/core";
import {
  ClipboardPaste,
  ClipboardType,
  Copy,
  Image as ImageIcon,
  Scissors,
  SquareDashed,
  Trash2,
} from "lucide-react";
import { BlockSideMenu } from "./BlockSideMenu";
import { EditorContextMenu } from "./EditorContextMenu";
import { SliteLinkToolbar } from "./EditorLinkToolbar";
import type { EditorMenuEntry } from "./EditorContextMenu";
import { onShow, resolveAttachmentUrl, uploadAttachment } from "../services/bridge";
import {
  applyPendingImageUrls,
  canReadPixels,
  copyImageToClipboard,
  imageFilesFrom,
  insertImageFiles,
  pasteFromClipboard,
  pastePlainText,
  pasteUrlAsLink,
  selectedImageUrl,
  urlOnlyFrom,
} from "../services/clipboard";
import { hasPrimaryModifier, SHORTCUT_MODIFIER } from "../services/platform";
import { t, useLocale } from "../services/i18n";
import type { Note } from "../types/note";

/**
 * The image block is enabled so that pasting a clipboard image or dropping an
 * image file on the editor resolves through the editor's uploadFile callback to
 * a content-addressed blob under attachments/, referenced relatively and served
 * by the AssetServer (see services/bridge.ts).
 *
 * There is deliberately no way to insert an image *from the UI*: the slash menu
 * entry and BlockNote's file panel are both disabled below, because the panel's
 * upload/embed tabs do not match how this app takes pictures (paste or drop,
 * never a URL) and the image is meant to arrive fully formed. The block spec must
 * stay in the schema regardless — BlockNote picks the block type for a pasted or
 * dropped file by matching `fileBlockAccept` against the schema. The other media
 * blocks (video / audio / file) stay excluded.
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
    image: defaultBlockSpecs.image,
  },
});

/** What the right-click menu acts on: where it opened, the image under the
 * cursor (or the selected one), and the block it sits in. */
interface EditorMenuState {
  x: number;
  y: number;
  imageSrc: string | null;
  blockId: string | null;
}

/**
 * Where a file dropped at (x, y) would land: the block under the pointer and
 * which side of it. The upper half of a block puts the picture before it and the
 * lower half after it — the rule BlockNote uses for its own drops. Only DOM
 * APIs: the block elements carry their id, so no editor internals are involved.
 */
function dropPlacementAt(
  x: number,
  y: number,
): { element: Element; reference: string; placement: "before" | "after" } | null {
  const block =
    document
      .elementFromPoint(x, y)
      ?.closest('[data-node-type="blockContainer"]') ?? nearestBlock(y);
  const reference = block?.getAttribute("data-id");
  if (!block || !reference) return null;
  const rect = block.getBoundingClientRect();
  return {
    element: block,
    reference,
    placement: rect.top + rect.height / 2 > y ? "before" : "after",
  };
}

/** The block closest to the pointer's height, used when the pointer is in the
 * empty space above or below the note, where no block is under it. */
function nearestBlock(y: number): Element | null {
  let best: Element | null = null;
  let bestDistance = Infinity;
  for (const block of document.querySelectorAll(
    '[data-node-type="blockContainer"][data-id]',
  )) {
    const rect = block.getBoundingClientRect();
    const distance = Math.abs(rect.top + rect.height / 2 - y);
    if (distance < bestDistance) {
      best = block;
      bestDistance = distance;
    }
  }
  return best;
}

/* Drop hints are plain classes on the block element: `dragover` fires for every
 * pointer move, and re-rendering the editor for each of them would be wasteful
 * for what is one painted line. */
const DROP_HINT_CLASSES: Record<"before" | "after", string> = {
  before: "slite-drop-before",
  after: "slite-drop-after",
};

function clearDropHint() {
  const classes = Object.values(DROP_HINT_CLASSES);
  document
    .querySelectorAll(`.${classes.join(",.")}`)
    .forEach((el) => el.classList.remove(...classes));
}

/* Keys handled at the window capture phase, as `KeyboardEvent.key` values from
 * the UI Events spec (https://www.w3.org/TR/uievents-key/). The DOM defines no
 * named constants for these (the keyCode ones are deprecated), so the spec
 * values are pinned here once instead of appearing as literals in handlers. */
const KEY_ENTER = "Enter";
const KEY_UNDO = "z";
const KEY_REDO = "y";

interface EditorProps {
  note: Note;
  blocknoteTheme: "light" | "dark";
  /** Fired on every document change; App debounces + persists. */
  onChange: (blocks: Block[]) => void;
  /**
   * Registers a markdown converter bound to this editor's schema. Called
   * with null when the editor unmounts (notes remount per note).
   */
  onConverterReady?: (converter: NoteConverter | null) => void;
}

/** Blocks ⇄ markdown, bound to an editor's schema (shared across notes). */
export interface NoteConverter {
  /** Blocks → markdown (lossy: tables flatten to plain text). */
  blocksToMarkdown: (blocks: Block[]) => string;
  /** Markdown → blocks, parsed with the current editor's schema. */
  markdownToBlocks: (markdown: string) => Block[];
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
export function Editor({ note, blocknoteTheme, onChange, onConverterReady }: EditorProps) {
  const locale = useLocale();
  // Only the failure path of `uploadFile` below needs this: BlockNote hands a
  // block id to `uploadFile` only on its own file-insertion paths, where it has
  // already created the block — a failed upload has to take it out again. Paste
  // and drop go through insertImageFiles instead, which takes the block back out
  // itself when nothing could be stored (see services/clipboard.ts).
  const editorRef = useRef<{ removeBlocks: (ids: string[]) => void } | null>(null);
  const editor = useCreateBlockNote({
    schema: sliteSchema,
    // BlockNote ships official locale dictionaries (zh from @blocknote/core/locales);
    // en is the built-in default. The App remounts this editor by note id + locale
    // so the dictionary is (re)applied on language switch.
    dictionary: locale === "zh-CN" ? zh : undefined,
    initialContent: (note.blocks?.length ? note.blocks : undefined) as
      | PartialBlock[]
      | undefined,
    // Trailing fake paragraph: fills the space below the content and turns
    // into a real block when clicked (see index.css for the full-height
    // styling).
    trailingBlock: true,
    // Clipboard image / dropped image file resolves through the attachment
    // pipeline (SaveAttachment → attachments/<hash>.<ext>); resolveFileUrl
    // turns that relative reference into a loadable /attachments/… path.
    uploadFile: async (file, blockId) => {
      try {
        return await uploadAttachment(file);
      } catch (err) {
        console.error("slite: image upload failed", err);
        if (blockId) editorRef.current?.removeBlocks([blockId]);
        throw err;
      }
    },
    resolveFileUrl: resolveAttachmentUrl,
    // Pasted images bypass BlockNote's own file insertion on purpose: it creates
    // the block first and then fills in the URL as a second document change,
    // which — once the upload outlives ProseMirror's 500 ms undo-grouping window
    // — made Ctrl+Z take the URL back off and leave a dead "Add image" block
    // behind. insertImageFiles keeps the block as the single undoable change and
    // writes the URL in outside the history. Everything else goes through
    // BlockNote's default handler untouched.
    pasteHandler: ({ event, editor: instance, defaultPasteHandler }) => {
      const files = imageFilesFrom(event.clipboardData);
      if (files.length) {
        void insertImageFiles(instance, files).catch((err) =>
          console.error("slite: pasting the image failed", err),
        );
        return true;
      }
      // A clipboard holding exactly one URL pastes as a link whose text is the
      // URL itself (see services/clipboard.ts). A payload with an HTML flavour
      // keeps BlockNote's own handling, and with text selected its "turn the
      // selection into a link" rule still applies — so fall through both times.
      const url = urlOnlyFrom(event.clipboardData);
      if (url && instance._tiptapEditor.state.selection.empty) {
        pasteUrlAsLink(instance, url);
        return true;
      }
      return defaultPasteHandler();
    },
  });
  editorRef.current = editor;

  // Expose the markdown converter to App for export/import. The schema is
  // shared by every note, so the converter stays valid for any note's blocks
  // even though the editor instance belongs to the active note.
  useEffect(() => {
    if (!onConverterReady) return;
    onConverterReady({
      blocksToMarkdown: (blocks) =>
        editor.blocksToMarkdownLossy(
          blocks as unknown as Parameters<typeof editor.blocksToMarkdownLossy>[0],
        ),
      markdownToBlocks: (md) => markdownToBlocks(md, editor._tiptapEditor.schema),
    });
    return () => onConverterReady(null);
  }, [editor, onConverterReady]);

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

  // Ctrl+Enter (Windows) / Cmd+Enter (macOS) toggles the checklist item
  // under the caret (Notion/Typora convention). BlockNote has no built-in
  // binding and the slash menu does not show this, so we handle it ourselves.
  // Capture phase: it must work even when ProseMirror has focus.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!hasPrimaryModifier(e) || e.altKey) return;

      // Undo/redo: repair image blocks that come back without their picture
      // (a paste's url fill is not part of the history — see
      // applyPendingImageUrls). Deferred, so ProseMirror applies the change
      // before we look at the document.
      const key = e.key.toLowerCase();
      if (key === KEY_UNDO || key === KEY_REDO) {
        setTimeout(() => applyPendingImageUrls(editor), 0);
      }

      if (e.key !== KEY_ENTER) return;
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

  // Ctrl+C on a selected image block copies the picture itself. BlockNote's own
  // copy handler writes text formats only (markdown in text/plain), and the
  // `<img src>` it embeds is a relative "attachments/…" reference that no
  // external app can resolve — so the pixels are written instead (see
  // services/clipboard.ts). Capture phase on the wrapper: it runs before
  // ProseMirror's own copy handler, so stopping propagation is enough to keep
  // BlockNote out of the way. Text selections are deliberately left to
  // BlockNote, and so are images whose bytes are not readable from here (a
  // remote URL from a pasted web image) — BlockNote's text copy puts their URL
  // on the clipboard, which beats copying nothing.
  const onCopyCapture = useCallback(
    (event: ClipboardEvent) => {
      const src = selectedImageUrl(editor);
      if (!src || !canReadPixels(src)) return;
      event.preventDefault();
      event.stopPropagation();
      void copyImageToClipboard(src).catch((err) =>
        console.error("slite: copying the image failed", err),
      );
    },
    [editor],
  );

  /* ------------------------------------------------------------------ */
  /* Right-click menu                                                     */
  /* ------------------------------------------------------------------ */

  // The window's built-in menu is Chromium's (emoji picker, writing direction,
  // "More tools"), and over the image element — which BlockNote marks
  // `contenteditable="false"` — Wails suppresses it entirely, so image blocks
  // had no menu at all. Both are replaced by components/EditorContextMenu.
  const [menu, setMenu] = useState<EditorMenuState | null>(null);

  const closeMenu = useCallback(() => setMenu(null), []);

  const onContextMenu = useCallback(
    (event: ReactMouseEvent<HTMLDivElement>) => {
      event.preventDefault();
      const target = event.target instanceof Element ? event.target : null;
      const imageBlock = target?.closest('[data-content-type="image"]');
      setMenu({
        x: event.clientX,
        y: event.clientY,
        // Falling back to the selection means a right click that misses the
        // picture by a few pixels still offers the image actions.
        imageSrc:
          imageBlock?.querySelector("img")?.getAttribute("src") ??
          selectedImageUrl(editor),
        blockId:
          target
            ?.closest('[data-node-type="blockContainer"]')
            ?.getAttribute("data-id") ?? null,
      });
    },
    [editor],
  );

  // Cut/copy run through `document.execCommand` rather than re-serializing the
  // selection: the browser then dispatches a real clipboard event, which is the
  // path BlockNote writes text with and the one the capture handler above
  // swaps pixels into for an image block. One implementation for the keyboard
  // shortcut and the menu entry.
  const runClipboardCommand = useCallback(
    (command: "cut" | "copy") => {
      editor.focus();
      document.execCommand(command);
    },
    [editor],
  );

  const copyImage = useCallback((src: string) => {
    void copyImageToClipboard(src).catch((err) =>
      console.error("slite: copying the image failed", err),
    );
  }, []);

  // Paste reads the clipboard itself: a paste event cannot be synthesized, and
  // BlockNote's own entry points (pasteHTML / pasteMarkdown / pasteText) are the
  // same ones its built-in paste handler calls.
  const paste = useCallback(() => {
    editor.focus();
    void pasteFromClipboard(editor).catch((err) =>
      console.error("slite: pasting failed", err),
    );
  }, [editor]);

  const pastePlain = useCallback(() => {
    editor.focus();
    void pastePlainText(editor).catch((err) =>
      console.error("slite: pasting failed", err),
    );
  }, [editor]);

  const selectAll = useCallback(() => {
    editor._tiptapEditor.commands.selectAll();
    editor.focus();
  }, [editor]);

  const deleteBlock = useCallback(
    (id: string) => {
      editor.removeBlocks([id]);
    },
    [editor],
  );

  // Entries are shaped by what was clicked: an image block gets the image
  // actions, anything else the text ones. Rebuilt on every render, which the
  // menu tolerates (it re-measures, and an unchanged position is a no-op).
  const menuEntries: EditorMenuEntry[] = [];
  if (menu) {
    const { imageSrc, blockId } = menu;
    if (imageSrc) {
      menuEntries.push({
        label: t.menuCopyImage,
        icon: <ImageIcon size={12} />,
        // A remote image has no bytes to read here; the entry still shows, so
        // the action is visibly unavailable rather than silently missing.
        disabled: !canReadPixels(imageSrc),
        onSelect: () => copyImage(imageSrc),
      });
      if (blockId) {
        menuEntries.push("separator", {
          label: t.menuDeleteBlock,
          icon: <Trash2 size={12} />,
          danger: true,
          onSelect: () => deleteBlock(blockId),
        });
      }
    } else {
      const hasSelection = !editor._tiptapEditor.state.selection.empty;
      menuEntries.push(
        {
          label: t.menuCut,
          icon: <Scissors size={12} />,
          shortcut: `${SHORTCUT_MODIFIER}X`,
          disabled: !hasSelection,
          onSelect: () => runClipboardCommand("cut"),
        },
        {
          label: t.menuCopy,
          icon: <Copy size={12} />,
          shortcut: `${SHORTCUT_MODIFIER}C`,
          disabled: !hasSelection,
          onSelect: () => runClipboardCommand("copy"),
        },
        {
          label: t.menuPaste,
          icon: <ClipboardPaste size={12} />,
          shortcut: `${SHORTCUT_MODIFIER}V`,
          onSelect: paste,
        },
        {
          label: t.menuPastePlain,
          icon: <ClipboardType size={12} />,
          onSelect: pastePlain,
        },
        "separator",
        {
          label: t.menuSelectAll,
          icon: <SquareDashed size={12} />,
          shortcut: `${SHORTCUT_MODIFIER}A`,
          onSelect: selectAll,
        },
      );
    }
  }

  /* ------------------------------------------------------------------ */
  /* Image drops                                                         */
  /* ------------------------------------------------------------------ */

  // A drop lands where the *pointer* is, not where the caret is, and that is
  // invisible until the mouse is released — which reads as "the picture went to
  // the wrong place". While a picture is dragged over the editor the block it
  // would land next to is marked, so the position is visible before letting go.
  // Same measurement as the drop below, so the hint cannot lie.
  const onDragOverCapture = useCallback(
    (event: ReactDragEvent<HTMLDivElement>) => {
      if (!imageFilesFrom(event.dataTransfer).length) return;
      const hit = dropPlacementAt(event.clientX, event.clientY);
      clearDropHint();
      if (hit) {
        hit.element.classList.add(DROP_HINT_CLASSES[hit.placement]);
      }
    },
    [],
  );

  // Dropped images take the same insert-then-fill path as pasted ones (see
  // insertImageFiles), so a drop is one document change too. The capture phase
  // keeps BlockNote's own file-drop insertion out of it. Only image-bearing
  // drops are taken over — a text drag inside the editor or a non-image file
  // keeps its previous behaviour.
  const onDropCapture = useCallback(
    (event: ReactDragEvent<HTMLDivElement>) => {
      const files = imageFilesFrom(event.dataTransfer);
      if (!files.length) return;
      event.preventDefault();
      event.stopPropagation();
      clearDropHint();

      const hit = dropPlacementAt(event.clientX, event.clientY);
      void insertImageFiles(editor, files, hit ?? undefined).catch((err) =>
        console.error("slite: inserting the dropped image failed", err),
      );
    },
    [editor],
  );

  return (
    <div
      className="h-full"
      onCopyCapture={onCopyCapture}
      onContextMenu={onContextMenu}
      onDragOverCapture={onDragOverCapture}
      onDragLeaveCapture={clearDropHint}
      onDropCapture={onDropCapture}
    >
      <BlockNoteView
        editor={editor}
        theme={blocknoteTheme}
        onChange={() => {
          // A redo puts an image block back without its url (the url fill is not
          // in the undo history), so repair pending pictures on every change.
          applyPendingImageUrls(editor);
          onChange(editor.document);
        }}
        sideMenu={false}
        linkToolbar={false}
        // No file panel and no slash-menu image entry: an image only ever arrives
        // by paste or drop here. BlockNote's panel offers an upload/embed tab
        // pair that does not fit a sticky note, and leaving it disabled without
        // dropping the entry too would produce a block whose "Add image" button
        // opens nothing.
        filePanel={false}
        slashMenu={false}
        data-testid="slite-editor"
      >
        <SideMenuController sideMenu={BlockSideMenu} />
        {/* Replaces the default link toolbar so a hovered link offers copying
            its text or its URL (see components/EditorLinkToolbar.tsx). */}
        <LinkToolbarController linkToolbar={SliteLinkToolbar} />
        <SuggestionMenuController
          triggerCharacter="/"
          shouldOpen={(state) =>
            !state.selection.$from.parent.type.isInGroup("tableContent")
          }
          getItems={async (query) =>
            filterSuggestionItems(
              getDefaultReactSlashMenuItems(editor).filter(
                // The item type drops `key`, but the runtime object still carries
                // the locale-independent discriminator from @blocknote/core —
                // filtering on that instead of a translated title.
                (item) => (item as { key?: string }).key !== "image",
              ),
              query,
            )
          }
        />
      </BlockNoteView>
      {menu && (
        <EditorContextMenu
          x={menu.x}
          y={menu.y}
          entries={menuEntries}
          onClose={closeMenu}
        />
      )}
    </div>
  );
}
