# Backlog

Queued feature requests, kept out of the README until they ship. The README
intentionally shows no roadmap; pick from here when planning the next release.

- Linux port (design shape reserved in the platform layer — ADR-0006; settle
  details in a grilling round when picked up)
- `scoop` / `winget` manifests
- IME candidate window occasionally lands at the screen's top-left corner
  instead of following the caret (seen once with the slash menu open; text
  still commits correctly, so DOM focus is fine). WebView2's TSF caret
  reporting fails intermittently at 125/150% display scaling — upstream
  WebView2Feedback #1611 / #2241, still open, runtime 152.0.4191.66 affected.
  No API exists to force a re-anchor; keep an eye on upstream.
- macOS: dropping image files does nothing. Wails intercepts file drags
  natively on macOS (`webview_window_darwin_drag.m` registers
  `NSFilenamesPboardType` on an overlay NSView), so the DOM `drop` event
  never carries the files — `onDropCapture` in Editor.tsx cannot see them
  and the paths go to Wails' `WindowFilesDropped` event, which is unused.
  Shipped in v0.9.2 (Go records the dropped images, `app:files-dropped`
  hands the release point to the editor, bytes come back via
  `LoadDroppedImages`). Pending real-hardware verification: drop placement
  accuracy (1 pt = 1 CSS px assumed), especially on scaled/external
  displays, and the hover highlight (the per-block hint stays Windows-only
  — the native path never fires DOM dragover there).
- macOS: the context-menu Paste uses `navigator.clipboard.read()`, whose
  WKWebView permission behaviour is unverified (keyboard Cmd+V is a real
  paste event and unaffected). Verify on a mac before relying on it.

## Rejected

- Slide in/out window animation (the "slide" in slite-note): conflicts with
  the `WS_EX_LAYERED` opacity path (`AnimateWindow` is a no-op on layered
  windows), a frontend CSS animation would only animate content while the
  window itself pops in/out instantly, and frame-by-frame `SetWindowPos`
  animation fights the opacity re-apply hooks and delays hide. Not worth it.
