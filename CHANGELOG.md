# Changelog

All notable changes to slite-note are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-08-19

### Added

- Single-instance guarantee: a second launch forwards its argv to the
  running instance instead of starting a second process (Wails
  `SingleInstance`). `slite-note.exe --quit` asks the running instance to
  flush pending saves and exit gracefully — used by the installer before
  upgrading. A plain second launch summons the hidden window.
- Product naming: shortcuts, install dir, uninstall entry and version
  resources now use **Slite Note** (the exe filename stays `slite-note.exe`);
  stable space-free uninstall key `zyitionSliteNote`.

### Changed

- NSIS installer hardened (fork of the Wails template):
  - Detects a running `slite-note.exe` before install/uninstall; prompts
    interactively, or gracefully quits it via `--quit` in silent mode
    (force-kill only as a bounded fallback).
  - Upgrades locate the previous install (new + legacy uninstall keys),
    reuse its directory, and run the old uninstaller first.
  - Interactive downgrades are blocked; silent installs overwrite.
  - Writes `InstallLocation` to the ARP registry key.
- Installer/ARP `DisplayVersion` now comes from the release tag
  (`windows:package VERSION=…`) instead of the hard-coded 0.1.0.
- Uninstall removes only the install dir and WebView2 cache — user notes in
  `%APPDATA%\slite` are kept.

### Fixed

- Overwrite install aborted with a file-in-use error when the previous
  version was still running.
- Silent installs no longer hang on invisible prompts (downgrade / running
  app) — `/SD` + `IfSilent` handling.

## [0.1.2] - 2026-08-19

### Changed

- WebView2 memory footprint: disable unused background features (Office
  Online web previews, built-in PDF viewer) and cap the V8 JS heap at
  128MB via the Windows browser options.

## [0.1.1] - 2026-08-19

### Fixed

- Floating toolbar buttons (bold, color, alignment, links) now work — the
  old click-anywhere handler stole focus from the toolbar and cleared the
  selection.
- Block drag-handle menu (Delete / Colors) is no longer squashed to 16px by
  an over-broad CSS rule.

### Added

- Caret continuity: the caret position is remembered when the window loses
  focus and restored on refocus or hotkey/tray summon, falling back to the
  document end.
- Trailing block fills the editor's remaining height — clicking below the
  content creates a new block.

### Changed

- Editor UI now uses BlockNote/Mantine defaults with theme tokens only;
  compact left/right gutters for the sticky-note layout; drag handle uses
  BlockNote's own 6-dot grip.

### Removed

- Media blocks (image / video / audio / file) from the schema until an
  attachment pipeline exists.

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

[Unreleased]: https://github.com/zyition/slite-note/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/zyition/slite-note/releases/tag/v0.2.0
[0.1.2]: https://github.com/zyition/slite-note/releases/tag/v0.1.2
[0.1.1]: https://github.com/zyition/slite-note/releases/tag/v0.1.1
[0.1.0]: https://github.com/zyition/slite-note/releases/tag/v0.1.0
