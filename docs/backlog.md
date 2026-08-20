# Backlog

Queued feature requests, kept out of the README until they ship. The README
intentionally shows no roadmap; pick from here when planning the next release.

- macOS port (design settled, see below; ADR/README/CONTEXT updates happen when implementation starts)
- `scoop` / `winget` manifests

## macOS port — settled design (2025-08, grilling session)

Goal: installable unsigned build (side-load, no Apple Developer account);
no Mac hardware — CI (macos runner) is the only build/verify path.

Decisions:

- **Shortcuts**: `Mod` abstraction (mac = Cmd, win = Ctrl) + per-conflict overrides.
  Quick Switch on mac = `Cmd+Shift+[` / `Cmd+Shift+]` (system `Ctrl+Tab` clash);
  global hotkey default stays `Alt+Shift+S` (Alt=Option, no clash). BlockNote's
  built-in `Mod`-based shortcuts need no change; our hardcoded `Ctrl` bindings do.
  `hotkey.ts` bug to fix: `metaKey` maps to `Super` (a Win key) — must be `Cmd` on mac.
- **App shape**: normal app (Dock + menu-bar icon); menu-bar single click opens the
  menu (mac convention, unlike Win tray left-click toggle). Close / `Cmd+W` hides,
  `Cmd+Q` quits. App menu: About / Settings… / Hide / Quit.
- **Opacity**: keep the slider; semantics differ per platform — Win = whole-window
  alpha (`WS_EX_LAYERED`), mac = background alpha via `Backdrop=Transparent` +
  `SetBackgroundColour(alpha)` + CSS background alpha (text stays crisp). No cgo.
- **Autostart**: replace registry Run-key code with cross-platform `app.Autostart`
  (`Arguments: ["--silent"]` for login-start quiet, matching Win semantics).
- **Dark mode / positioning / single-instance / tray**: Wails v3 cross-platform
  paths; darwin impl needed for bounds persistence (mind Retina logical vs
  physical px) and Downloads dir / OpenURL.
- **Packaging**: Universal Binary (arm64+amd64, lipo) + dmg via CI, ad-hoc signed;
  side-load steps (`xattr -cr`) documented at release time.
- **Architecture**: `internal/platform` package (build tags) abstracting window
  ops / opacity / system paths; keep it 3-platform shaped but only implement
  Windows + macOS (Linux extensibility only).

Phases (when picked up): 0 platform abstraction + hotkey.ts fix → 1 frontend
shortcut platformisation → 2 macOS system integration → 3 CI Universal dmg →
4 release + docs. Low-priority: CI smoke test (`--smoke` arg, verify launch).

Risks (no real hardware): native window behaviour (positioning/focus/menu bar)
and mac transparency visuals (3 themes × alpha) unverifiable locally — rely on
CI smoke + user feedback; mac global-hotkey conflict detection is unreliable
(OS allows duplicate registration); Wails v3 beta.9 mac paths may have bugs.

## Rejected

- Slide in/out window animation (the "slide" in slite-note): conflicts with
  the `WS_EX_LAYERED` opacity path (`AnimateWindow` is a no-op on layered
  windows), a frontend CSS animation would only animate content while the
  window itself pops in/out instantly, and frame-by-frame `SetWindowPos`
  animation fights the opacity re-apply hooks and delays hide. Not worth it.
