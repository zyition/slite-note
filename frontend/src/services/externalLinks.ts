/**
 * External links open in the user's default browser — never in a second WebView
 * window.
 *
 * BlockNote routes every link through the global `window.open`: the link mark's
 * click handler (@blocknote/core Link/clickHandler) and the link toolbar's
 * "open" button both call it. WebView2 turns that into another app window with
 * no frame, no toolbar and no way back — so `window.open` is replaced here with
 * the native open (Go's OpenURL → ShellExecute on Windows / `open` on macOS; a
 * new tab in the browser fallback). Plain anchors are prevented from navigating
 * the main window away the same way.
 *
 * The WebView must never navigate away from the app either: a link that slips
 * past ProseMirror's click handler would otherwise replace the note UI with a
 * web page.
 */
import { openUrl } from "./bridge";

/** Schemes that belong to the user's browser or mail client. Anything else
 * (relative paths, `#` anchors, `javascript:`) is not ours to open. */
const EXTERNAL_SCHEME = /^(?:https?|ftps?|mailto|tel):/i;

function openExternal(href: string) {
  void openUrl(href).catch((err) =>
    console.error("slite: opening the link failed", err),
  );
}

/** The anchor a DOM event happened on, when it points somewhere external. */
function anchorFrom(event: Event): HTMLAnchorElement | null {
  const target = event.target;
  if (!(target instanceof Element)) return null;
  const anchor = target.closest<HTMLAnchorElement>("a[href]");
  if (!anchor || !EXTERNAL_SCHEME.test(anchor.href)) return null;
  return anchor;
}

let installed = false;

/** Install the external-link handling. Idempotent, called once at startup. */
export function installExternalLinkHandling(): void {
  if (installed) return;
  installed = true;

  window.open = ((url?: string | URL) => {
    const href = url instanceof URL ? url.href : String(url ?? "");
    if (href) openExternal(href);
    return null; // no WebView window, ever
  }) as typeof window.open;

  // Left clicks on anchors *outside* the editor. The editor's own links are
  // handled by ProseMirror's click handler (which ends up in the patched
  // `window.open` above), so skipping them here also avoids opening twice.
  document.addEventListener(
    "click",
    (event) => {
      const anchor = anchorFrom(event);
      if (!anchor || anchor.closest(".bn-editor")) return;
      event.preventDefault();
      openExternal(anchor.href);
    },
    true,
  );

  // Middle click: ProseMirror ignores non-left buttons, so the anchor's default
  // action would let WebView2 open its own window.
  document.addEventListener(
    "auxclick",
    (event) => {
      if (event.button !== 1) return;
      const anchor = anchorFrom(event);
      if (!anchor) return;
      event.preventDefault();
      event.stopPropagation();
      openExternal(anchor.href);
    },
    true,
  );
}
