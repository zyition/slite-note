# Changelog

All notable changes to slite-note are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-08-19

### Fixed

- Custom data directory no longer resets to the default after a restart: the
  migration (`SetDataDir`) used to delete the default dir's `settings.json`,
  which is the only record of the custom `DataDir`, so any restart (including
  an overwrite install) fell back to `%APPDATA%\slite`. The default dir's
  settings.json is now kept in sync as a DataDir anchor.
- Auto-start state is now read back correctly: `getLaunchAtStartup` queried
  the registry value name `slite` while `setLaunchAtStartup` writes
  `slite-note`, so the toggle always showed off after restart. Both names are
  honored (legacy `slite` entries are cleaned up on disable).

### Changed

- Open-source readiness cleanup: removed the internal spec (`req.md`), dropped
  the China-only npm mirror from `frontend/.npmrc`, and unified the copyright
  year to 2026 across license and build metadata.

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

[Unreleased]: https://github.com/zyition/slite-note/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/zyition/slite-note/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/zyition/slite-note/releases/tag/v0.2.0
