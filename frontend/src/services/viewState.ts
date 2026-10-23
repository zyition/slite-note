/**
 * In-memory handoff of the editor view across a lightweight-mode teardown.
 *
 * When the hidden window is destroyed, its WebView (and with it the
 * ProseMirror selection, the DOM scroll position and the editor instance) is
 * gone. Go asks for this snapshot before closing and replays it after the
 * window is recreated. Nothing is persisted: quitting the app forgets it,
 * which matches "remember within the process" semantics.
 */

interface EditorLike {
  _tiptapEditor: {
    state: {
      selection: { anchor: number; head: number };
      doc: { content: { size: number } };
    };
    commands: {
      setTextSelection: (selection: number | { from: number; to: number }) => void;
    };
  };
  focus: () => void;
}

interface Registration {
  noteId: string;
  editor: EditorLike;
}

interface ViewState {
  noteId: string;
  anchor: number;
  head: number;
  scrollTop: number;
}

let registered: Registration | null = null;

/** Called by the mounted Editor; returns an unregister function. */
export function registerViewState(noteId: string, editor: EditorLike): () => void {
  const registration = { noteId, editor };
  registered = registration;
  return () => {
    // A remount may already have registered its successor.
    if (registered === registration) registered = null;
  };
}

function scrollElement(): HTMLElement | null {
  return document.querySelector<HTMLElement>("[data-note-scroll]");
}

function clampPosition(value: unknown, max: number): number {
  const n = Math.trunc(Number(value));
  if (!Number.isFinite(n)) return 0;
  return Math.min(Math.max(n, 0), max);
}

/** Opaque JSON snapshot for Go's app:flush-before-destroy acknowledgement. */
export function captureViewState(): string {
  const reg = registered;
  const el = scrollElement();
  if (!reg || !el) return "";
  try {
    const selection = reg.editor._tiptapEditor.state.selection;
    const state: ViewState = {
      noteId: reg.noteId,
      anchor: selection.anchor,
      head: selection.head,
      scrollTop: el.scrollTop,
    };
    return JSON.stringify(state);
  } catch {
    return "";
  }
}

/** Replay a snapshot from app:restore-view-state. Returns false when there is
 * no matching editor (the note changed or the page was restarted). */
export function applyViewState(raw: string): boolean {
  const reg = registered;
  const el = scrollElement();
  if (!reg || !el) return false;

  let state: ViewState;
  try {
    state = JSON.parse(raw) as ViewState;
  } catch {
    return false;
  }
  if (!state || state.noteId !== reg.noteId) return false;

  const pm = reg.editor._tiptapEditor;
  const max = pm.state.doc.content.size;
  const anchor = clampPosition(state.anchor, max);
  const head = clampPosition(state.head, max);
  try {
    pm.commands.setTextSelection(
      anchor === head ? anchor : { from: Math.min(anchor, head), to: Math.max(anchor, head) },
    );
    reg.editor.focus();
  } catch {
    return false;
  }

  const top = Math.max(0, Number(state.scrollTop) || 0);
  const restoreScroll = () => {
    el.scrollTop = top;
  };
  // focus() may scroll the caret into view; set the saved position once after
  // that, and again after the next layout pass.
  restoreScroll();
  requestAnimationFrame(restoreScroll);
  return true;
}
