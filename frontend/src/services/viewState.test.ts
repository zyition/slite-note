import { afterEach, describe, expect, it, vi } from "vitest";
import { applyViewState, captureViewState, registerViewState } from "./viewState";

function fakeEditor(anchor = 4, head = 7) {
  const setTextSelection = vi.fn();
  const focus = vi.fn();
  return {
    editor: {
      _tiptapEditor: {
        state: {
          selection: { anchor, head },
          doc: { content: { size: 20 } },
        },
        commands: { setTextSelection },
      },
      focus,
    },
    setTextSelection,
    focus,
  };
}

function installDom(scrollTop = 0) {
  const scrollEl = { scrollTop } as HTMLElement;
  vi.stubGlobal("document", { querySelector: () => scrollEl });
  vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
    cb(0);
    return 1;
  });
  return scrollEl;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("viewState", () => {
  it("captures the selection and scroll position", () => {
    installDom(123);
    const { editor } = fakeEditor(4, 7);
    registerViewState("note-1", editor);

    expect(JSON.parse(captureViewState())).toEqual({
      noteId: "note-1",
      anchor: 4,
      head: 7,
      scrollTop: 123,
    });
  });

  it("restores a range selection and scroll position", () => {
    const scrollEl = installDom(0);
    const { editor, setTextSelection, focus } = fakeEditor(0, 0);
    registerViewState("note-1", editor);

    const restored = applyViewState(
      JSON.stringify({ noteId: "note-1", anchor: 4, head: 7, scrollTop: 123 }),
    );

    expect(restored).toBe(true);
    expect(setTextSelection).toHaveBeenCalledWith({ from: 4, to: 7 });
    expect(focus).toHaveBeenCalled();
    expect(scrollEl.scrollTop).toBe(123);
  });

  it("restores a collapsed caret as a position", () => {
    installDom();
    const { editor, setTextSelection } = fakeEditor(0, 0);
    registerViewState("note-1", editor);

    applyViewState(JSON.stringify({ noteId: "note-1", anchor: 6, head: 6, scrollTop: 0 }));

    expect(setTextSelection).toHaveBeenCalledWith(6);
  });

  it("ignores snapshots for another note and clamps stale positions", () => {
    installDom();
    const { editor, setTextSelection } = fakeEditor(0, 0);
    registerViewState("note-1", editor);

    expect(
      applyViewState(JSON.stringify({ noteId: "note-2", anchor: 1, head: 1, scrollTop: 0 })),
    ).toBe(false);
    expect(setTextSelection).not.toHaveBeenCalled();

    applyViewState(JSON.stringify({ noteId: "note-1", anchor: 999, head: 999, scrollTop: 0 }));
    expect(setTextSelection).toHaveBeenCalledWith(20);
  });
});
