# Changelog

All notable changes to slite-note are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-08-19

### Added

- Initial public release.
- Global hotkey (`Alt+Shift+S`, reconfigurable) to hide/summon the window.
- Close = hide; tray icon with show/hide and quit.
- Always-on-top pin and window opacity (50–100%), persisted.
- Theme follows system light/dark, or manual sticky-note yellow, persisted.
- Silent auto-start (`--silent`): launch at sign-in without popping the
  window.
- BlockNote editor: `/` slash menu, checklists, headings, quotes, code
  blocks, drag-to-reorder, bubble toolbar.
- Auto-save (800ms debounce) to a single local JSON file.
- Multiple notes: title-bar picker, create, delete.
- Relocatable data directory (Settings → *Change location…*).
- WebView2 user data relocated to `%LOCALAPPDATA%\slite\webview`.
- Window: frameless custom title bar, 1/3-width left-edge placement on the
  work area (raw Win32 `SetWindowPos`), first-line-centered block drag handle.
- Browser fallback mode for UI development (no Wails runtime needed).

### Fixed

- Duplicate tray icon (SystemTray auto-run vs explicit `Run`).
- Startup window occasionally centering instead of hugging the left edge
  (re-position after WebView2 settles).
- Opacity reset after window resize/move (Wails re-applies alpha).
- BlockNote heading sizes and slash-menu theming in both themes.
- Default WebView2 data path (`%APPDATA%\slite.exe` dir) replaced with a
  clean LocalAppData location.

[Unreleased]: https://github.com/zyition/slite-note/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/zyition/slite-note/releases/tag/v0.1.0
