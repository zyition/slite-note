import { describe, expect, it, vi } from "vitest";
import {
  insertImageFiles,
  pasteUrlAsLink,
  selectedImageUrl,
  urlOnlyFrom,
} from "./clipboard";

/*
 * `selectedImageUrl` is the lookup the copy interceptor and the right-click
 * menu share, and the part that is easy to get wrong: a node-selected block
 * puts the DOM selection *around* the block element, so the browser's own
 * `window.getSelection()` never points inside it. These cases pin down the
 * ProseMirror shapes that mean "the user selected this image" and the ones
 * that must keep BlockNote's own clipboard handling in charge.
 */

interface FakeNode {
  type: { name: string };
  attrs: Record<string, unknown>;
  firstChild: FakeNode | null;
}

function node(
  name: string,
  attrs: Record<string, unknown> = {},
  firstChild: FakeNode | null = null,
): FakeNode {
  return { type: { name }, attrs, firstChild };
}

function fragment(children: FakeNode[]): {
  childCount: number;
  child: (index: number) => FakeNode;
} {
  return { childCount: children.length, child: (index) => children[index] };
}

/** Stand-in for whatever shape the selection takes. */
function editorWith(selection: unknown) {
  return { _tiptapEditor: { state: { selection } } } as never;
}

const nodeSelection = (block: FakeNode) => ({
  empty: false,
  node: block,
  content: () => ({ content: fragment([block]) }),
});

const rangeSelection = (blocks: FakeNode[]) => ({
  empty: false,
  content: () => ({ content: fragment(blocks) }),
});

const collapsed = (block: FakeNode) => ({
  empty: true,
  content: () => ({ content: fragment([block]) }),
});

const image = node("image", { url: "attachments/abc123.png" });
const imageContainer = node("blockContainer", { id: "b1" }, image);
const paragraph = node("paragraph", { id: "b2" }, node("text"));

describe("selectedImageUrl", () => {
  it("reads the image of a node-selected image block", () => {
    expect(selectedImageUrl(editorWith(nodeSelection(image)))).toBe(
      "attachments/abc123.png",
    );
  });

  it("reads the image through a node-selected block container", () => {
    // What BlockNote expands the selection to before copying an image block.
    expect(selectedImageUrl(editorWith(nodeSelection(imageContainer)))).toBe(
      "attachments/abc123.png",
    );
  });

  it("reads the image of a range selection covering that one block", () => {
    expect(selectedImageUrl(editorWith(rangeSelection([imageContainer])))).toBe(
      "attachments/abc123.png",
    );
  });

  it("ignores a range selection spanning more than one block", () => {
    expect(
      selectedImageUrl(editorWith(rangeSelection([imageContainer, paragraph]))),
    ).toBeNull();
  });

  it("ignores a range selection inside a non-image block", () => {
    expect(selectedImageUrl(editorWith(rangeSelection([paragraph])))).toBeNull();
  });

  it("ignores a collapsed selection", () => {
    expect(selectedImageUrl(editorWith(collapsed(imageContainer)))).toBeNull();
  });

  it("ignores an image block without a URL", () => {
    expect(selectedImageUrl(editorWith(nodeSelection(node("image"))))).toBeNull();
  });
});

/*
 * A URL-only clipboard is pasted as a self-link, not with a copied
 * hyperlink's anchor label (see the URLs note in clipboard.ts). These cases pin
 * down what counts as "exactly one URL" and the escaping of the anchor that is
 * built from it.
 */
describe("urlOnlyFrom", () => {
  const clipboard = (text: string, types: string[] = ["text/plain"]) =>
    ({ getData: () => text, types }) as unknown as DataTransfer;

  it("reads a lone URL, ignoring surrounding whitespace", () => {
    expect(urlOnlyFrom(clipboard("  https://baidu.com\n"))).toBe(
      "https://baidu.com",
    );
    expect(urlOnlyFrom(clipboard("mailto:hi@example.com"))).toBe(
      "mailto:hi@example.com",
    );
  });

  it("ignores an empty clipboard", () => {
    expect(urlOnlyFrom(clipboard("   "))).toBeNull();
    expect(urlOnlyFrom(null)).toBeNull();
  });

  it("ignores text that merely contains a URL", () => {
    expect(
      urlOnlyFrom(clipboard("see https://baidu.com for details")),
    ).toBeNull();
  });

  it("ignores a URL without a scheme (BlockNote autolinks those itself)", () => {
    expect(urlOnlyFrom(clipboard("www.baidu.com"))).toBeNull();
    expect(urlOnlyFrom(clipboard("baidu.com/path"))).toBeNull();
  });

  it("ignores a clipboard that also carries HTML", () => {
    // A copied hyperlink keeps BlockNote's own HTML handling (and the anchor's
    // label) — only a text-only clipboard is taken over.
    expect(
      urlOnlyFrom(clipboard("https://baidu.com", ["text/plain", "text/html"])),
    ).toBeNull();
  });
});

describe("pasteUrlAsLink", () => {
  function fakeEditor() {
    const pasted: string[] = [];
    return {
      editor: { pasteHTML: (html: string) => pasted.push(html) } as never,
      pasted,
    };
  }

  it("pastes a link whose text is the URL itself", () => {
    const { editor, pasted } = fakeEditor();
    pasteUrlAsLink(editor, "https://baidu.com");
    expect(pasted).toEqual([
      '<p><a href="https://baidu.com">https://baidu.com</a></p>',
    ]);
  });

  it("escapes the URL so it cannot break out of the anchor", () => {
    const { editor, pasted } = fakeEditor();
    pasteUrlAsLink(editor, 'https://x.test/?a=1&b="<>"');
    expect(pasted).toEqual([
      '<p><a href="https://x.test/?a=1&amp;b=&quot;&lt;&gt;&quot;">' +
        'https://x.test/?a=1&amp;b="&lt;&gt;"</a></p>',
    ]);
  });
});

/*
 * Pasting and dropping an image must be one document change. BlockNote's own
 * file insertion creates the block first and sets the URL when the upload
 * resolves; two changes that a slow upload can push outside ProseMirror's 500 ms
 * undo-grouping window, which is how a Ctrl+Z ended up leaving an empty "Add
 * image" block behind. These cases pin the order down.
 */
describe("insertImageFiles", () => {
  const file = { name: "pasted.png", type: "image/png" } as File;

  function fakeEditor(options: { hasContent?: boolean; fail?: boolean } = {}) {
    const calls: string[] = [];
    const inserted: any[] = [];
    const insertArgs: unknown[][] = [];
    const updated: { id: string; update: unknown }[] = [];
    const props: Record<string, string> = {};
    const editor = {
      uploadFile: async () => {
        calls.push("upload");
        if (options.fail) throw new Error("no space left on device");
        return "attachments/abc123.png";
      },
      focus: () => {},
      getTextCursorPosition: () => ({
        block: {
          id: "b1",
          type: "paragraph",
          content: options.hasContent ? [{ type: "text" }] : [],
        },
      }),
      updateBlock: (id: string, update: unknown) => {
        calls.push("update");
        updated.push({ id, update });
        Object.assign(props, (update as { props: Record<string, string> }).props);
        return { id };
      },
      getBlock: (id: string) =>
        id === "b1" || id.startsWith("n") ? { id, props } : undefined,
      transact: (fn: (tr: { setMeta: (k: string, v: boolean) => void }) => void) =>
        fn({ setMeta: () => {} }),
      insertBlocks: (blocks: any[], reference: unknown, placement: unknown) => {
        calls.push("insert");
        insertArgs.push([reference, placement]);
        inserted.push(...blocks);
        return blocks.map((_block, index) => ({ id: `n${index}` }));
      },
      replaceBlocks: (_removed: unknown, blocks: any[]) => {
        calls.push("replace");
        inserted.push(...blocks);
        return {
          insertedBlocks: blocks.map((_block, index) => ({ id: `n${index}` })),
        };
      },
      removeBlocks: () => calls.push("remove"),
    };
    return { editor: editor as never, calls, inserted, insertArgs, updated };
  }

  it("replaces an empty block in one step, then fills the URL in", async () => {
    const { editor, calls, updated } = fakeEditor();
    await expect(insertImageFiles(editor, [file])).resolves.toBe(true);
    // The replace is the single undoable document change; the URL fill follows
    // it (outside the history) and must not precede the upload.
    expect(calls).toEqual(["replace", "upload", "update"]);
    expect(updated).toEqual([
      { id: "n0", update: { props: { url: "attachments/abc123.png" } } },
    ]);
  });

  it("inserts after the current block when it has content", async () => {
    const { editor, calls, insertArgs, updated } = fakeEditor({
      hasContent: true,
    });
    await expect(insertImageFiles(editor, [file])).resolves.toBe(true);
    expect(calls).toEqual(["insert", "upload", "update"]);
    expect(insertArgs[0][1]).toBe("after");
    expect(updated).toEqual([
      { id: "n0", update: { props: { url: "attachments/abc123.png" } } },
    ]);
  });

  it("places a dropped image on the requested side of a block", async () => {
    const { editor, calls, insertArgs } = fakeEditor();
    await expect(
      insertImageFiles(editor, [file], { reference: "b7", placement: "before" }),
    ).resolves.toBe(true);
    expect(calls).toEqual(["insert", "upload", "update"]);
    expect(insertArgs[0]).toEqual(["b7", "before"]);
  });

  it("takes the block back out when the upload fails", async () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    const { editor, calls, updated } = fakeEditor({ fail: true });
    await expect(insertImageFiles(editor, [file])).resolves.toBe(true);
    expect(calls).toEqual(["replace", "upload", "remove"]);
    expect(updated).toEqual([]);
    expect(error).toHaveBeenCalled();
    error.mockRestore();
  });
});
