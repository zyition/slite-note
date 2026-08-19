# slite-note

A minimal sticky-note desktop app for Windows. One slim window hosts a
block-based rich-text editor (BlockNote); notes are stored locally as a single
JSON file. The window hides and summons with a global hotkey — an always-ready
scratchpad that stays out of your way.

> This project is not affiliated with [slite.com](https://slite.com).

[中文说明](./README.zh-CN.md)

## Features

- **Global hotkey** (`Alt+Shift+S` by default, reconfigurable) toggles the
  window from anywhere — no focus needed
- **Close = hide**: the app stays resident in the system tray; quit from the
  tray menu
- **Always on top** pin, **opacity** (50–100%), theme following the OS light /
  dark mode (or a manual sticky-note yellow)
- **Silent auto-start**: optional launch at sign-in without popping the window
- **Block editor**: BlockNote — `/` slash menu, checklists, headings, quotes,
  code blocks, drag-to-reorder, bubble toolbar
- **Auto-save** (800ms debounce) to a single local file
- **Multiple notes** via the title-bar picker; delete from the note menu
- **Relocatable data**: move your notes folder from the settings panel
- **Zero telemetry**: everything stays on your machine

## Install

- **Windows 10/11** with the [WebView2 runtime]
  (preinstalled on most systems; the installer bundles it when missing).

### Options

| Channel | How |
|---|---|
| GitHub Releases | `slite-note-amd64-installer.exe` (NSIS installer) or portable `slite-note-<ver>-windows-amd64.zip` |
| From source | see [Building](#building) |

> The portable zip needs WebView2 already installed. The installer will set it
> up automatically on older machines.

## Building

Toolchain: Go 1.25+, Node/pnpm, and the [Wails v3 CLI].

```bash
cd frontend && pnpm install     # frontend deps
wails3 build                    # → bin/slite-note.exe
```

> On Windows, ensure `PACKAGE_MANAGER=pnpm` and `wails3` (plus mise shims) are
> on `PATH`.

### Browser-only UI development

Without the Wails runtime the app falls back to `localStorage` + no-op window
controls, so the UI can be developed in a plain browser:

```bash
cd frontend && pnpm dev         # http://localhost:9245
```

## Data

| What | Where |
|---|---|
| Notes | `%APPDATA%\slite\notes.json` (single file, relocatable via Settings → *Change location…*) |
| Settings | `%APPDATA%\slite\settings.json` |
| WebView2 data | `%LOCALAPPDATA%\slite\webview` |
| Logs | `%APPDATA%\slite\log.txt` (debug only) |

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
