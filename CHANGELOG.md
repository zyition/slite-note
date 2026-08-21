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
