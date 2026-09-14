## [0.10.1] - 2026-09-14

### Bug Fixes

- *(windows)* Resolve startup background colour from the saved theme

### Performance

- *(windows)* Reclaim the process working set after the note is hidden

### Documentation

- Update AGENTS.md for the Windows + macOS platform split
## [0.10.0] - 2026-09-11

### Features

- *(tray)* Localize tray menu labels
- *(editor)* Ctrl/Cmd+Shift+V pastes as plain text

### Bug Fixes

- *(attachments)* Serve attachment responses with Cache-Control no-store
- *(editor)* Drop replace/caption/rename/download from the formatting toolbar
- *(tray)* Drop the settings menu entry
## [0.9.2] - 2026-09-11

### Features

- *(editor)* Drop image files onto the editor on macOS

### Documentation

- Mark the macOS image-drop backlog item shipped in v0.9.2
## [0.9.0] - 2026-09-11

### Features

- Paste images into notes as local file attachments
- *(editor)* Paste, drop and copy images without BlockNote's file panel
- *(ui)* Remember the open note across restarts (device-local)
- *(editor)* Style links so they read as links
- *(editor)* Paste a bare URL as a self-link
- *(editor)* Add copy link and copy text to the link toolbar

### Bug Fixes

- *(editor)* Open links in the system browser, not a new WebView
## [0.8.1] - 2026-08-25

### Bug Fixes

- *(ci)* Bump upload/download-artifact to Node 24 majors

### Documentation

- Document pasted-code loss and standardise app name
## [0.8.0] - 2026-08-24

### Features

- *(ui)* Add UI font-size scale, themed accent and refined controls
## [0.7.2] - 2026-08-21

### Bug Fixes

- *(ui)* Space out modifier keys in settings hotkey row
- *(darwin)* Bundle the app as "Slite Note.app" so Finder/Spotlight show the right name
## [0.7.1] - 2026-08-21

### Bug Fixes

- *(mac)* Correct startup window placement, tray, icon and first-launch opacity
## [0.7.0] - 2026-08-21

### Features

- *(ui)* Add Simplified Chinese localization with language picker

### Bug Fixes

- *(ui)* Normalize user-facing app name to "Slite Note"
## [0.6.1] - 2026-08-21

### Performance

- *(frontend)* Cache note titles per-note on edits
## [0.6.0] - 2026-08-20

### Features

- MacOS port — platform layer, Mod shortcuts, background alpha

### Testing

- Make shortcut/hotkey tests host-independent

### Documentation

- MacOS port ADRs, install notes, backlog design
- Drop shipped macOS port from backlog
## [0.5.0] - 2026-08-20

### Features

- *(testing)* Add vitest unit tests and wire them into CI
- *(installer)* Launch after install and optional user-data removal
- *(ui)* Rework titlebar layout and add Ctrl+N for new note

### Bug Fixes

- *(version)* Inject appVersion via ldflags instead of source rewrite
- *(installer)* Use MUI2 PageEx for the uninstall data page

### Refactoring

- *(win32)* Extract pure window logic into internal/windowutil
- *(store)* Decouple data-dir pointer from settings
- *(store)* Store notes as one file per note

### Documentation

- *(testing)* Add e2e test plan
## [0.4.0] - 2026-08-19

### Features

- Window bounds memory and markdown import/export
- *(export)* Folder picker for markdown export, defaulting to Downloads
## [0.3.0] - 2026-08-19

### Features

- Add shortcut cheatsheet and new keyboard shortcuts

### Documentation

- Remove roadmap and drop slide-animation mentions
## [0.2.1] - 2026-08-19

### Bug Fixes

- *(store)* Keep custom data dir across restarts; fix autostart registry key

### Documentation

- Drop changelog entries for removed 0.1.x releases
## [0.2.0] - 2026-08-19

### Features

- Theme picker popover, system follows dark/gray, overlays stay opaque
- Single-instance guard, hardened NSIS installer, Slite Note branding

### Bug Fixes

- *(editor)* Floating toolbar clicks, caret memory, official UI styles

### Performance

- Trim WebView2 background features and cap V8 JS heap

### Documentation

- ADR 0004 — per-note file storage + attachments directory
