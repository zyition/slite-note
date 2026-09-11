/**
 * Clipboard plumbing for the editor: copying an image block out as real
 * pixels, and the paste actions the custom context menu needs.
 *
 * Copy. BlockNote's own copy handler writes text formats only, and the
 * `<img src>` it embeds is a relative "attachments/<hash>.<ext>" reference no
 * external app can resolve — so Ctrl+C on an image block used to put nothing
 * usable on the clipboard. Intercepting the copy event lets us write the actual
 * pixels instead: the same bytes end up on the clipboard as if the image had
 * been copied from a browser, and an image-only clipboard entry pastes back
 * into the note through the normal attachment pipeline.
 *
 * Which image is selected comes from ProseMirror's selection rather than
 * `window.getSelection()`: a node-selected block puts the DOM selection *around*
 * the block element (its range starts in the block container), and the trailing
 * whitespace selection the browser keeps for a node selection would never match
 * the image element either. The editor's own selection state is exact.
 *
 * Paste. The context menu replaces the WebView2 built-in menu, and a trusted
 * paste event cannot be synthesized — so the menu reads the clipboard through
 * the async Clipboard API and drives the same BlockNote entry points the
 * built-in paste handler uses (`pasteHTML` / `pasteMarkdown` / `pasteText`).
 */

import type {
  Block,
  BlockNoteEditor,
  PartialBlock,
} from "@blocknote/core";

/** Any BlockNote editor instance (the schema is the caller's business). */
type Editor = BlockNoteEditor<any, any, any>;

/*
 * Structural views of the ProseMirror bits we touch. Importing the real types
 * would mean depending on prosemirror-model/state directly, which are
 * transitive dependencies of @blocknote/core, not ours.
 */

/** A ProseMirror node, narrowed to what an image block lookup needs. */
interface BlockNode {
  type: { name: string };
  attrs: Record<string, unknown>;
  firstChild: BlockNode | null;
}

/** The subset of `EditorState.selection` used below. */
interface SelectionLike {
  empty: boolean;
  /** Set on a node selection (drag handle, or a click on the block). */
  node?: BlockNode;
  content: () => {
    content: { childCount: number; child: (index: number) => BlockNode };
  };
}

/**
 * Chromium only accepts `image/png` for programmatic clipboard writes, so every
 * other attachment format (jpg/gif/webp) is re-encoded through a canvas. Static
 * images only: an animated GIF loses its animation, which is an acceptable
 * trade for "copy one picture" in a sticky note.
 */
async function asPNG(blob: Blob): Promise<Blob> {
  if (blob.type === "image/png") return blob;

  const bitmap = await createImageBitmap(blob);
  const canvas = document.createElement("canvas");
  canvas.width = bitmap.width;
  canvas.height = bitmap.height;
  const context = canvas.getContext("2d");
  if (!context) throw new Error("no 2D canvas context");
  context.drawImage(bitmap, 0, 0);
  bitmap.close();

  const png = await new Promise<Blob | null>((resolve) =>
    canvas.toBlob(resolve, "image/png"),
  );
  if (!png) throw new Error("could not encode the image as PNG");
  return png;
}

/** The image URL inside a block node: the image block content itself, or the
 * block container wrapping it. */
function imageUrlIn(node: BlockNode | null | undefined): string | null {
  if (!node) return null;
  const image = node.type.name === "image" ? node : node.firstChild;
  if (!image || image.type.name !== "image") return null;
  const url = image.attrs.url;
  return typeof url === "string" && url ? url : null;
}

/**
 * The image the editor's selection covers, as its stored reference
 * ("attachments/<hash>.<ext>", a remote URL or a data URL), or null when the
 * selection is not a single image block.
 *
 * Three shapes count as "the selected image": a node selection on the image
 * block (BlockNote's drag handle), a node selection on the block container
 * wrapping it (what BlockNote expands the selection to before copying), and a
 * range selection covering exactly that one block (the user drag-selecting
 * across the picture). Requiring the range to cover a single block keeps text
 * selections out of the copy interceptor.
 */
export function selectedImageUrl(editor: Editor): string | null {
  const selection = editor._tiptapEditor.state
    .selection as unknown as SelectionLike;

  if (selection.node) return imageUrlIn(selection.node);

  if (selection.empty) return null;
  const fragment = selection.content().content;
  if (fragment.childCount !== 1) return null;
  return imageUrlIn(fragment.child(0));
}

/**
 * Whether the image bytes can be read from this page: a local attachment
 * (same origin, served by the AssetServer) or an inline data/blob URL. Remote
 * images are deliberately excluded — fetching them would need CORS and a
 * download step the app does not have, and BlockNote's own copy already writes
 * their URL as text, so the user gets something useful either way. (Downloading
 * remote images into attachments/ is a separate feature, not this one.)
 */
export function canReadPixels(src: string): boolean {
  if (src.startsWith("data:") || src.startsWith("blob:")) return true;
  try {
    return new URL(src, window.location.href).origin === window.location.origin;
  } catch {
    return false;
  }
}

/**
 * Fetch an image (attachment URL, data URL or blob URL) and put its pixels on
 * the system clipboard. Resolves to false when the image cannot be read.
 */
export async function copyImageToClipboard(src: string): Promise<boolean> {
  const response = await fetch(src);
  if (!response.ok) return false;
  const png = await asPNG(await response.blob());
  await navigator.clipboard.write([new ClipboardItem({ "image/png": png })]);
  return true;
}

/*
 * Paste actions — see the file header for why these read the clipboard
 * themselves instead of relying on a paste event.
 */

/** Clipboard read that never throws: an unreadable clipboard (no permission,
 * unfocused window, an exotic format only) just behaves like an empty one. */
async function readClipboard(): Promise<ClipboardItems> {
  try {
    return await navigator.clipboard.read();
  } catch (err) {
    console.error("slite: reading the clipboard failed", err);
    return [];
  }
}

async function readTextClipboard(): Promise<string> {
  try {
    return await navigator.clipboard.readText();
  } catch (err) {
    console.error("slite: reading the clipboard failed", err);
    return "";
  }
}

/** The first image on the clipboard as a File, or null. */
async function clipboardImage(items: ClipboardItems): Promise<File | null> {
  for (const item of items) {
    const type = item.types.find((t) => t.startsWith("image/"));
    if (!type) continue;
    const blob = await item.getType(type);
    const extension = type.split("/")[1] ?? "png";
    return new File([blob], `pasted.${extension}`, { type: blob.type });
  }
  return null;
}

/** The image files in a paste/drop payload. Non-images are left to BlockNote:
 * this schema enables no file block for them to land in. */
export function imageFilesFrom(data: DataTransfer | null): File[] {
  return Array.from(data?.files ?? []).filter((file) =>
    file.type.startsWith("image/"),
  );
}

/** Upload one image through the editor's pipeline: Go's SaveAttachment in
 * native mode (content-addressed blob under attachments/), a data URL in the
 * browser fallback. Resolves to null when the upload failed. */
async function uploadImageFile(
  editor: Editor,
  file: File,
): Promise<string | null> {
  try {
    const uploaded = await editor.uploadFile?.(file);
    return typeof uploaded === "string" ? uploaded : null;
  } catch (err) {
    console.error("slite: image upload failed", err);
    return null;
  }
}

/**
 * Image blocks whose picture was uploaded: block id → url. Entries are kept for
 * the whole session (id → short string) because the history can bring a removed
 * block back: an undo takes the block out, and a redo puts it back *without* its
 * url — the fill is not part of the history — so it has to be repaired from
 * here. Pruning entries would leave that redo with an empty placeholder for
 * good.
 */
const pendingImageUrls = new Map<string, string>();

/**
 * Write in the url of an image block whose bytes are on disk — *outside* the undo
 * history.
 *
 * ProseMirror only folds two document changes into one undo step while they keep
 * arriving within `newGroupDelay` (500 ms) and stay adjacent, so BlockNote's own
 * "insert the block now, set the url when the upload resolves" splits into two
 * undo steps as soon as the upload is slow: the first Ctrl+Z then took the url
 * back off and left a dead "Add image" block behind (that button opens the file
 * panel, which this app disables). `addToHistory: false` keeps a paste one
 * undoable change no matter how long the upload takes. BlockNote's `transact`
 * reuses the active transaction, so the nested `updateBlock` lands in this one
 * and inherits the meta.
 *
 * Called for every uploaded picture that is missing its own url — right after an
 * upload resolves and again on every document change: undoing a paste takes the
 * whole block out, and redoing it puts the block back *without* its url, which
 * would leave the empty placeholder behind. Blocks that are currently gone are
 * skipped, so their entry stays available for a later redo.
 */
export function applyPendingImageUrls(editor: Editor) {
  if (!pendingImageUrls.size) return;
  editor.transact((tr) => {
    tr.setMeta("addToHistory", false);
    for (const [id, url] of pendingImageUrls) {
      const block = editor.getBlock(id);
      if (!block) continue; // undone: leave the url for a possible redo
      if ((block.props as { url?: string }).url !== url) {
        editor.updateBlock(id, { props: { url } });
      }
    }
  });
}

/**
 * Insert `blocks` at the caret, replacing the current block when that is still
 * empty — BlockNote's own rule for a pasted file (see handleFileInsertion in
 * @blocknote/core), implemented with a step ProseMirror can undo.
 *
 * BlockNote's own version calls `updateBlock`, and switching an *empty* block
 * from a paragraph to an image goes through ProseMirror's `setNodeMarkup`: that
 * emits a `replaceAround` step whose inverse cannot be applied to a node that
 * holds no content. The history records the change, the inverse fails silently,
 * and Ctrl+Z leaves the picture exactly where it was — which is what pasting
 * into an empty block (the usual case) looked like. Remove-and-insert is a plain
 * replace step, so the whole gesture stays undoable.
 */
function insertAtCaret(
  editor: Editor,
  blocks: PartialBlock<any, any, any>[],
): Block<any, any, any>[] {
  const current = editor.getTextCursorPosition().block;
  const isEmpty = Array.isArray(current.content) && current.content.length === 0;
  if (!isEmpty) return editor.insertBlocks(blocks, current, "after");
  return editor.replaceBlocks([current.id], blocks).insertedBlocks;
}

/**
 * Insert an image block per file and fill the pictures in as their bytes are
 * written.
 *
 * The block goes in *before* the upload, on purpose: the document change has to
 * land while the paste or drop is still the newest thing the user did. Uploading
 * first (which an earlier version did) put the change a few hundred milliseconds
 * later, and a user who pressed Ctrl+Z straight after pasting undid the change
 * *before* the paste instead — the picture then appeared on top of it, i.e. "the
 * paste did not get undone". With the block inserted first that Ctrl+Z removes
 * it, and the url fill stays out of the history (see applyPendingImageUrls), so the whole
 * gesture is one undo step.
 *
 * `target` places the block at a drop location (the block it was released over,
 * and which side of it); without one the block lands at the caret.
 */
export async function insertImageFiles(
  editor: Editor,
  files: File[],
  target?: { reference: string; placement: "before" | "after" },
): Promise<boolean> {
  if (!files.length) return false;

  const placeholders = files.map((file) => ({
    type: "image",
    props: { name: file.name },
  })) as PartialBlock<any, any, any>[];

  const inserted: Block<any, any, any>[] = target
    ? editor.insertBlocks(placeholders, target.reference, target.placement)
    : insertAtCaret(editor, placeholders);
  if (!inserted.length) return false;

  // A drop arrives from outside the window, so the focus is on the page body
  // afterwards and Ctrl+Z (or typing) would never reach the editor.
  editor.focus();

  await Promise.all(
    inserted.map(async (block, index) => {
      const url = await uploadImageFile(editor, files[index]);
      if (!url) {
        // Nothing to show for it: take the empty block back out.
        editor.removeBlocks([block.id]);
        return;
      }
      pendingImageUrls.set(block.id, url);
      applyPendingImageUrls(editor);
    }),
  );
  return true;
}

/**
 * Paste the clipboard at the caret, following BlockNote's own priority (see
 * `acceptedMIMETypes` in @blocknote/core): an image first, then HTML, then
 * plain text — which the default paste handler parses as Markdown. Resolves to
 * false when the clipboard held nothing we can insert.
 */
export async function pasteFromClipboard(editor: Editor): Promise<boolean> {
  const items = await readClipboard();

  const file = await clipboardImage(items);
  if (file) return insertImageFiles(editor, [file]);

  for (const item of items) {
    if (!item.types.includes("text/html")) continue;
    editor.pasteHTML(await (await item.getType("text/html")).text());
    return true;
  }

  for (const item of items) {
    if (!item.types.includes("text/plain")) continue;
    editor.pasteMarkdown(await (await item.getType("text/plain")).text());
    return true;
  }

  // Some clipboards expose text only through the simpler API.
  const text = await readTextClipboard();
  if (text) {
    editor.pasteMarkdown(text);
    return true;
  }
  return false;
}

/**
 * Paste the clipboard as literal text — no HTML, no Markdown parsing. This is
 * BlockNote's code-block paste path, and the workaround for the documented
 * "pasting code can silently drop content" issue (see README, Known Issues).
 */
export async function pastePlainText(editor: Editor): Promise<boolean> {
  const items = await readClipboard();
  for (const item of items) {
    if (!item.types.includes("text/plain")) continue;
    editor.pasteText(await (await item.getType("text/plain")).text());
    return true;
  }

  const text = await readTextClipboard();
  if (!text) return false;
  editor.pasteText(text);
  return true;
}
