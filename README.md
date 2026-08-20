# slite-note

A minimal sticky-note desktop app for Windows and macOS. One slim window hosts a
block-based rich-text editor (BlockNote); notes are stored locally as a single
JSON file. The window hides and summons with a global hotkey — an always-ready
scratchpad that stays out of your way.

> This project is not affiliated with [slite.com](https://slite.com).

[中文说明](./README.zh-CN.md)

## Features

- **Global hotkey** (`Alt+Shift+S` by default, reconfigurable) toggles the
  window from anywhere — no focus needed
- **Close = hide**: the app stays resident (system tray on Windows, menu bar on
  macOS); quit from the tray/menu or `Cmd+Q` (macOS)
- **Always on top** pin, **opacity** (50–100%), theme following the OS light /
  dark mode (or a manual sticky-note yellow)
- **Window position memory**: size and placement survive restarts; a window
  left on a disconnected monitor falls back to the default spot
- **Silent auto-start**: optional launch at sign-in without popping the window
- **Block editor**: BlockNote — `/` slash menu, checklists, headings, quotes,
  code blocks, drag-to-reorder, bubble toolbar
- **Markdown import/export**: export a note as `.md` from the picker, import a
  `.md` as a new note, or export every note from Settings
- **Auto-save** (800ms debounce) to a single local file
- **Multiple notes** via the title-bar picker; delete from the note menu
- **Relocatable data**: move your notes folder from the settings panel
- **Zero telemetry**: everything stays on your machine

## Install

- **Windows 10/11** with the [WebView2 runtime]
  (preinstalled on most systems; the installer bundles it when missing).
- **macOS 13+** (Apple Silicon or Intel). No Developer-ID signature: the first
  launch shows a Gatekeeper "unidentified developer" warning — right-click the
  app → **Open**, or run `xattr -cr /Applications/slite-note.app` once.

### Options

| Channel | How |
|---|---|
| GitHub Releases (Windows) | `slite-note-amd64-installer.exe` (NSIS installer) or portable `slite-note-<ver>-windows-amd64.zip` |
| GitHub Releases (macOS) | `slite-note.dmg` — drag `slite-note.app` into Applications (Universal: Apple Silicon + Intel) |
| From source | see [Building](#building) |

> The portable zip needs WebView2 already installed. The installer will set it
> up automatically on older machines.

### Platform notes

- **Shortcuts** use `Cmd` on macOS and `Ctrl` on Windows (⌘B/⌘N/⌘, …). Note
  switching: `Ctrl+Tab` / `Ctrl+Shift+Tab` on Windows, `Cmd+Shift+]` /
  `Cmd+Shift+[` on macOS (system `Ctrl+Tab`/`Cmd+Tab` stay reserved). The
  cheatsheet (`Mod+Shift+/`) shows the current platform's combos.
- **Opacity** means the whole window fades on Windows, while on macOS only the
  note background becomes see-through (text stays crisp).
- The macOS menu bar adds **Settings…** (`Cmd+,`), and `Cmd+W` hides the window
  like the close button; `Cmd+Q` quits.

## Building

Toolchain: Go 1.25+, Node/pnpm, and the [Wails v3 CLI].

```bash
cd frontend && pnpm install     # frontend deps
wails3 build                    # → bin/slite-note.exe (Windows)
wails3 task darwin:build        # → bin/slite-note (macOS; run on macOS)
```

> On Windows, ensure `PACKAGE_MANAGER=pnpm` and `wails3` (plus mise shims) are
> on `PATH`.

> macOS builds must run on macOS (cgo/ObjC bridge — no cross-compilation).
> Release CI builds a Universal `slite-note.dmg` (arm64 + amd64, ad-hoc
> signed) via `wails3 task darwin:package:universal:dmg`.

### Browser-only UI development

Without the Wails runtime the app falls back to `localStorage` + no-op window
controls, so the UI can be developed in a plain browser:

```bash
cd frontend && pnpm dev         # http://localhost:9245
```

## Data

| What | Where |
|---|---|
| Notes | `%APPDATA%\slite\notes.json` on Windows / `~/Library/Application Support/slite/notes.json` on macOS (single file, relocatable via Settings → *Change location…*) |
| Settings | `%APPDATA%\slite\settings.json` on Windows / `~/Library/Application Support/slite/settings.json` on macOS |
| WebView data | `%LOCALAPPDATA%\slite\webview` (Windows) |
| Logs | `log.txt` inside the data dir (debug only) |

## Privacy

slite-note is fully offline. No telemetry, no analytics, no network calls at
runtime. Your notes never leave your machine.

## Tech stack

| Layer | Choice | Notes |
|---|---|---|
| Desktop shell | **Wails v3** | [ADR-0001](./docs/adr/0001-wails-v3-instead-of-neutralino.md) |
| Frontend | Vite + React 19 + TypeScript + Tailwind v4 | CSS-first, no tailwind config |
| Editor | BlockNote 0.54 | `/` menu, drag handle, bubble toolbar |
| Icons | lucide-react | |

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) and [AGENTS.md](./AGENTS.md).

## Acknowledgments

Built with [Wails], [BlockNote], [React], [Tailwind CSS] and
[lucide]. Full third-party notices in [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md).

## License

[MIT](./LICENSE) © 2026 zyition

[WebView2 runtime]: https://developer.microsoft.com/en-us/microsoft-edge/webview2/
[Wails v3 CLI]: https://wails.io
[Wails]: https://wails.io
[BlockNote]: https://www.blocknotejs.org
[React]: https://react.dev
[Tailwind CSS]: https://tailwindcss.com
[lucide]: https://lucide.dev
