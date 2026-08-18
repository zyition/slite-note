# AGENTS.md — slite-note

Guidance for AI agents (and humans) working in this repository.

## Project

slite-note: minimal Windows sticky-note desktop app. Wails v3 (Go) shell +
Vite/React 19/TypeScript/Tailwind v4 frontend + BlockNote editor. Data lives
locally in `%APPDATA%\slite\`. English-first UI strings; maintainers
communicate in Chinese.

## Toolchain (Windows host)

- Dev happens on **Windows** (WSL2 hosts the terminal; all tool invocations
  must go through Windows tools via `pwsh.exe` — never WSL binaries).
- Package managers via mise: `node`, `pnpm`, `go` (Windows installs).
- Mirrors (do not change): Go `GOPROXY=https://goproxy.cn,direct`;
  npm/pnpm registry `https://registry.npmmirror.com` (`frontend/.npmrc`).
- wails3 CLI: `%USERPROFILE%\.local\bin\wails3.exe`.

## Build / verify

```powershell
$env:PATH = "$env:LOCALAPPDATA\mise\shims;$env:USERPROFILE\.local\bin;" + $env:PATH
$env:PACKAGE_MANAGER = 'pnpm'
wails3 build          # → bin\slite-note.exe
```

- Bindings regenerate with `wails3 generate bindings -ts -i` (after Go API
  changes); output lands in `frontend/bindings/github.com/zyition/slite-note/`
  — do not hand-edit.
- **Quality gate (mandatory before commit)**: `go vet .` (main package only —
  `go build ./...` fails on `build/ios`, a cross-compile placeholder) and
  `pnpm exec tsc --noEmit` in `frontend/`.
- **No automated e2e tests.** The maintainer manually verifies native
  features (tray, hotkey, window placement). The browser fallback
  (`cd frontend && pnpm dev`, port 9245, host `0.0.0.0`) is used for UI
  verification from the WSL side via `http://172.28.176.1:9245`.

## Layout

```
├── main.go            # Wails app: window/tray/hotkey/events (Win32 interop)
├── store.go           # Store service (bindings): notes + settings persistence
├── tools/genicon/     # Icon generator (Go, image/png)
├── frontend/src/
│   ├── App.tsx        # orchestration: boot/save/theme/new/switch
│   ├── components/    # TitleBar / NotePicker / Editor / SettingsPanel / BlockSideMenu
│   ├── services/      # bridge / i18n / theme / title / hotkey
│   └── types/note.ts  # Note / Settings domain types
└── docs/adr/          # decision records
```

## Key invariants (read the code before touching)

- **Data dir is stable**: `%APPDATA%\slite\` (and `%LOCALAPPDATA%\slite\webview`,
  `log.txt`) intentionally keeps the `slite` name even though the product is
  slite-note — renaming would orphan existing user data.
- **Close = hide, tray = quit, hotkey = summon.** Do not make close quit.
- `--silent` launch (auto-start) must never pop the window.
- Window is positioned with raw Win32 `SetWindowPos` after WebView2
  navigation completes (see `positionWindowAtStartup`/`showMainWindowAtStartup`).
- Opacity is applied via `WS_EX_LAYERED` + `SetLayeredWindowAttributes` and
  must be re-applied on window resize/move (Wails resets alpha).
- Side-menu handle alignment is measured at runtime
  (`BlockSideMenu.tsx`) — heading rows use ported BlockNote offsets; keep
  first-line centering exact.
- UI strings: English-first (see `services/i18n.ts`); no hardcoded Chinese in
  the UI.

## Conventions

- Chinese for agent–maintainer communication; English in code, comments and UI.
- ADRs live in `docs/adr/` (format in `docs/adr/*.md`). Glossary in
  `CONTEXT.md` — keep it implementation-free.
