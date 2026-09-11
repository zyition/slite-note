import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { ReactNode } from "react";

/**
 * EditorContextMenu — the editor's own right-click menu.
 *
 * The window runs in WebView2, whose built-in menu is Chromium's: emoji picker,
 * writing direction, "More tools" and a web-inspector entry, none of which mean
 * anything here. Wails only steps aside for it (`--default-contextmenu`), and
 * for the `<img>` BlockNote marks `contenteditable="false"` it suppresses the
 * menu entirely, so image blocks had no menu at all. Replacing it keeps the
 * entries context-shaped: text and image selections get different lists.
 *
 * Mousedown is swallowed before it can move focus (or clear the selection), so
 * the editor keeps its caret and the block the user right-clicked on.
 */

export interface EditorMenuItem {
  label: string;
  icon?: ReactNode;
  /** Right-aligned hint, e.g. "Ctrl+V". */
  shortcut?: string;
  /** Destructive action (delete). */
  danger?: boolean;
  /** Greyed out; the action cannot run in the current selection. */
  disabled?: boolean;
  onSelect: () => void;
}

/** A menu is items in order, with `"separator"` drawing a group divider. */
export type EditorMenuEntry = EditorMenuItem | "separator";

interface EditorContextMenuProps {
  /** Viewport coordinates of the click that opened the menu. */
  x: number;
  y: number;
  entries: EditorMenuEntry[];
  /** Close request: outside click, Escape, blur, resize or scroll. */
  onClose: () => void;
}

const EDGE_GAP = 4;

export function EditorContextMenu({
  x,
  y,
  entries,
  onClose,
}: EditorContextMenuProps) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  // Positioned at the cursor, then pulled back inside the viewport once the
  // menu has been measured (its height depends on the entry list).
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  useLayoutEffect(() => {
    const el = rootRef.current;
    if (!el) return;
    const { width, height } = el.getBoundingClientRect();
    // Idempotent: the effect also runs when the entry list is rebuilt, and
    // re-setting an equal position would re-render forever.
    setPos((prev) => {
      const next = {
        left: Math.max(
          EDGE_GAP,
          Math.min(x, window.innerWidth - width - EDGE_GAP),
        ),
        top: Math.max(
          EDGE_GAP,
          Math.min(y, window.innerHeight - height - EDGE_GAP),
        ),
      };
      return prev && prev.left === next.left && prev.top === next.top
        ? prev
        : next;
    });
  }, [x, y, entries]);

  // Dismissal: the menu is transient, so anything that invalidates its anchor
  // closes it. Captured `mousedown` covers clicks that never reach a click
  // handler (a right click elsewhere, the tray, another window).
  useEffect(() => {
    const close = () => closeRef.current();
    const onDown = (event: MouseEvent) => {
      if (rootRef.current?.contains(event.target as Node)) return;
      close();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("mousedown", onDown, true);
    window.addEventListener("keydown", onKey);
    window.addEventListener("blur", close);
    window.addEventListener("resize", close);
    window.addEventListener("wheel", onDown, true);
    return () => {
      window.removeEventListener("mousedown", onDown, true);
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("blur", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("wheel", onDown, true);
    };
  }, []);

  return createPortal(
    <div
      ref={rootRef}
      role="menu"
      // Keeps the editor's focus and selection: without this, mousedown on the
      // menu would collapse the selection the action is about to act on.
      onMouseDown={(event) => event.preventDefault()}
      onContextMenu={(event) => event.preventDefault()}
      className="fixed z-[100] min-w-44 rounded-md border border-[var(--border)] bg-[var(--bg)] p-1 shadow-lg"
      style={{
        left: pos?.left ?? x,
        top: pos?.top ?? y,
        // Hidden for the single frame before the measurement lands.
        visibility: pos ? "visible" : "hidden",
      }}
    >
      {entries.map((entry, index) =>
        entry === "separator" ? (
          <div
            key={`sep-${index}`}
            className="mx-1 my-1 h-px bg-[var(--border)]"
          />
        ) : (
          <button
            key={entry.label}
            type="button"
            role="menuitem"
            disabled={entry.disabled}
            onClick={() => {
              closeRef.current();
              entry.onSelect();
            }}
            className={`flex w-full items-center gap-2 rounded px-1.5 py-1 text-left text-[length:var(--fs-body)] ${
              entry.disabled
                ? "cursor-default text-[var(--fg-muted)] opacity-60"
                : entry.danger
                  ? "text-red-500 hover:bg-[var(--hover)]"
                  : "hover:bg-[var(--hover)]"
            }`}
          >
            <span className="flex h-[length:var(--icon-sm)] w-[length:var(--icon-sm)] shrink-0 items-center justify-center">
              {entry.icon}
            </span>
            <span className="min-w-0 flex-1 truncate">{entry.label}</span>
            {entry.shortcut && (
              <span className="shrink-0 text-[length:var(--fs-tiny)] text-[var(--fg-muted)]">
                {entry.shortcut}
              </span>
            )}
          </button>
        ),
      )}
    </div>,
    document.body,
  );
}
