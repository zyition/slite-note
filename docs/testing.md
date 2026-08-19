# e2e Testing Plan

Status: **planned — not implemented.** Unit tests (vitest for the frontend,
`go test` for `internal/windowutil` + the store) cover the pure logic; this
document lists the end-to-end scenarios that need a real window / real OS
integration and where they should run.

## Why two tracks

The app has two runtimes:

1. **Browser fallback** (`cd frontend && pnpm dev`, port 9245) — the full UI
   on a plain browser with `localStorage` persistence and no-op window
   controls. Fully automatable with Playwright on any OS.
2. **Native Windows** (Wails + WebView2 + Win32) — tray, global hotkey,
   opacity, window placement, auto-start, single-instance, the file store.
   Automating WebView2 content requires the WinAppDriver + WebView2 WebDriver
   hybrid stack (WebView2's debugging port is disabled in production builds),
   which is a significant upfront investment; these are manual for now.

Rule of thumb: anything reachable through the DOM goes in track 1; anything
that needs Win32 or the Go bindings goes in track 2.

## Track 1 — browser fallback (Playwright, automatable)

Tooling: Playwright (already standard in the Vite ecosystem). Run against
`pnpm dev` on port 9245 with the `localStorage` adapter (no Wails runtime →
`bridge.ts` detects non-native).

### P0 — core note lifecycle

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T1-1 | First-run seeding | Fresh profile (clear localStorage) → open app | One empty note exists; editor focused |
| T1-2 | Typing persists | Type text, wait >800ms, reload | Text restored from localStorage |
| T1-3 | Multiple notes | New note ×2, switch via picker | Each note keeps its own content; active note highlighted |
| T1-4 | Delete keeps one | Delete all notes | Exactly one empty note remains; editor still works |
| T1-5 | Auto-save error banner | Make `saveNotes` reject (intercept in test) | Error banner shows for ~4s |

### P0 — editor basics (BlockNote)

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T1-6 | Slash menu | Type `/` | Command palette opens; selecting a heading applies it |
| T1-7 | Checklist | Insert checklist, press Ctrl+Enter | Item toggles checked |
| T1-8 | Bold shortcut | Ctrl+B on selection | Bold applied (marks stored in blocks) |
| T1-9 | Title derivation | First line of a note changes | Picker shows derived title (≤28 chars, ellipsis) |

### P1 — settings & import/export

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T1-10 | Theme picker | Switch theme yellow/gray/dark | `document.documentElement.dataset.theme` + swatch update |
| T1-11 | Settings open/close | Ctrl+, then ✕ | Panel opens; window opacity override fires (no-op in browser, logged) |
| T1-12 | Export single note | Note menu → Export as Markdown | Browser downloads `<title>.md` |
| T1-13 | Import markdown | Import a `.md` fixture | New note appended with parsed blocks |
| T1-14 | Export all | Settings → Export all | One `.md` per note downloaded |

### P2 — shortcuts & cheatsheet

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T1-15 | Ctrl+Shift+T cycles themes | Press repeatedly | system → dark → gray → yellow → system |
| T1-16 | Ctrl+Tab / Ctrl+Shift+Tab | Two notes, press both | Switches notes in both directions; Alt+Tab unaffected |
| T1-17 | Cheatsheet | Ctrl+Shift+/ | Modal lists shortcuts; Esc closes |

## Track 2 — native Windows (manual checklist)

Automation (WinAppDriver + WebView2 WebDriver) is a later investment; until
then these are a manual regression checklist for each release.

### P0 — must pass before any release

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T2-1 | Fresh install | Run installer, launch | Window appears at left edge, 1/3 screen width, note yellow bg |
| T2-2 | Close = hide | Alt+F4 / ✕ | Window hides, app stays in tray; process alive |
| T2-3 | Tray summon | Click tray icon / tray menu Show | Window shows focused, caret at end of note |
| T2-4 | Global hotkey | Default Alt+Shift+S from another app | Window toggles system-wide |
| T2-5 | Quit via tray | Tray → Quit | Process exits; no orphan |
| T2-6 | Hotkey reconfig | Settings → Change → record Ctrl+K | New combo toggles; old combo dead; failure path shows error |
| T2-7 | Save flush on hide | Type, immediately hide, wait, reopen | Last keystrokes persisted (800ms debounce flushed) |

### P1 — should pass

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T2-8 | Window bounds memory | Move/resize, quit, relaunch | Window reopens at saved bounds |
| T2-9 | Off-screen fallback | Save bounds on disconnected monitor, relaunch | Window falls back to primary work area |
| T2-10 | Opacity | Set 50%, relaunch; move window | Window translucent; opacity survives move/resize and restart |
| T2-11 | Always on top | Pin, alt-tab to another app | Note stays on top; unpin releases |
| T2-12 | Silent auto-start | Enable launch at startup, sign out/in (or launch exe --silent) | No window pops; tray present; hotkey summons |
| T2-13 | Single instance | Launch app twice | Second launch summons the first instance, no second process |
| T2-14 | Data dir migration | Settings → move data to empty folder | Notes relocated; restart keeps new dir (anchor survives) |
| T2-15 | Markdown export native | Export to Downloads | Files written with sanitized names; collision suffix `(2)` |
| T2-16 | Upgrade path | Install vX, add note, install vX+1 | Notes intact; old auto-start registry entry migrated |

### P2 — nice to have (per release if time permits)

| # | Scenario | Steps | Expect |
|---|---|---|---|
| T2-17 | Corrupt notes.json | Replace file with garbage, launch | App boots with empty note; `.corrupt-*` backup exists |
| T2-18 | Read-only data dir | chmod / remove write permission, launch | App boots, save shows error banner |
| T2-19 | High-DPI / scaling | 150% scaling, relaunch | Window placed correctly; opacity re-applied after DPI resize |
| T2-20 | WebView2 missing | Uninstall runtime, launch installer | Installer bootstraps WebView2 |

## Tooling notes (when track 2 automation lands)

- **WinAppDriver** (Microsoft) drives the native window chrome; WebView2
  content needs the WebView2 WebDriver (`msedgedriver`) enabled via
  `--remote-debugging-port` — check whether Wails exposes a debug flag rather
  than shipping it in production.
- **agent-browser** can cover track 1 today (it drives real browsers); the
  browser fallback is the same DOM, so track 1 cases port directly.
- Keep the `P0` sets green for every release; `P1` for every minor bump;
  `P2` ad hoc.
