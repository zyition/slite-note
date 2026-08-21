package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/zyition/slite-note/internal/windowutil"
)

// Wails embeds the built frontend into the binary (frontend/dist is produced by
// `pnpm build` / `wails3 dev`).
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed icons/tray.png
var trayIcon []byte

// App-wide events forwarded to the frontend so it can flush pending saves and
// open the settings panel from the tray.
func init() {
	application.RegisterEvent[string]("app:hide")
	application.RegisterEvent[string]("app:show")
	application.RegisterEvent[string]("app:quit")
	application.RegisterEvent[string]("app:open-settings")
}

const (
	defaultHotkey = "Alt+Shift+S"
	initialWidth  = 480 // startup width; adjusted to 1/3 of the screen in positionWindowAtStartup
	minWidth      = 320
	minHeight     = 320
)

// activeHotkey is the currently registered toggle accelerator; empty while
// suspended or until the first successful registration.
var activeHotkey = ""

// suspendedHotkey remembers the combo unregistered by suspendToggleHotkey so
// resumeToggleHotkey can restore it (unless a new combo was set meanwhile).
var suspendedHotkey = ""

// opacityOverride forces the window fully opaque while an app-modal overlay
// (settings panel, theme picker) is open; while set, the persisted opacity is
// not re-applied by SaveSettings or the window-change hooks.
var opacityOverride = false

// debugLog writes diagnostics to %APPDATA%/slite/log.txt (kept small: useful
// while the app is headless). Release builds stay silent — no log.txt on user
// machines; SLITE_DEBUG=1 re-enables it for a session, and dev builds always
// log.
func debugLog(format string, args ...any) {
	if appVersion != "dev" && os.Getenv("SLITE_DEBUG") == "" {
		return
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return
	}
	path := filepath.Join(cfg, "slite", "log.txt")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}

var (
	app        *application.App
	mainWindow *application.WebviewWindow
	store      *Store
)

// webviewDataPath returns a tidy WebView2 user-data location (%LOCALAPPDATA%
// /slite/webview), or "" to let the runtime pick a default when LocalAppData
// is unavailable.
func webviewDataPath() string {
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		return filepath.Join(d, "slite", "webview")
	}
	return ""
}

func main() {
	// --silent (auto-start): the window stays hidden after startup; the user
	// summons it via the global hotkey or the tray.
	// --smoke (CI): boot, create+show the window, verify, then exit.
	for _, a := range os.Args {
		switch a {
		case "--silent":
			silentStart = true
		case "--smoke":
			smokeMode = true
		}
	}

	store = NewStore()

	app = application.New(application.Options{
		Name:        "slite-note",
		Description: "A minimal desktop sticky notes app",
		Services: []application.Service{
			application.NewService(store),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Single-instance guard: a second launch forwards its argv to the
		// running instance instead of starting a second process. Two processes
		// would race on notes.json (the store only mutexes within one process),
		// and the window is designed to be hidden, so a second launch should
		// summon it. Windows implementation: named mutex + hidden message-only
		// window + WM_COPYDATA (see single_instance_windows.go in Wails).
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.zyition.slite-note",
			// The installer runs "slite-note.exe --quit" before upgrading so
			// the running instance can flush pending saves and exit gracefully
			// (releasing the exe file lock) instead of being force-killed.
			OnSecondInstanceLaunch: onSecondInstanceLaunch,
			ExitCode:               0,
		},
		Windows: application.WindowsOptions{
			// Default is %APPDATA%/<exe>.exe which is both ugly and roaming;
			// put WebView2's user data (EBWebView cache) under LocalAppData.
			WebviewUserDataPath: webviewDataPath(),
			// Trim WebView2's background features we never use: Office Online
			// web previews (msWebOOUI) and the built-in PDF viewer (msPdfOOUI).
			// msSmartScreenProtection is already disabled by Wails by default.
			// (Sticky notes render only local HTML; see WebView2 performance docs.)
			DisabledFeatures: []string{
				"msWebOOUI",
				"msPdfOOUI",
			},
			// Cap the V8 JS heap so it cannot balloon beyond what a note editor
			// needs (BlockNote/ProseMirror is heavy; keep >= 96MB to avoid GC
			// churn). No GPU flags: hardware acceleration must stay on.
			AdditionalBrowserArgs: []string{
				"--js-flags=--max-old-space-size=128",
			},
		},
	})

	settings := store.currentSettings()
	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Slite Note",
		Width:            initialWidth,
		Height:           480,
		MinWidth:         280,
		MinHeight:        320,
		Frameless:        true,
		AlwaysOnTop:      settings.AlwaysOnTop,
		BackgroundColour: application.NewRGB(255, 243, 176), // note yellow (default theme)
		URL:              "/",
		// Start hidden; shown after the startup positioning below to avoid a flash.
		Hidden: true,
		// macOS: make the window shell transparent so the frontend's semi-
		// transparent note background (--bg-opacity) shows the desktop through.
		// Ignored on Windows (which uses WS_EX_LAYERED for opacity instead).
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTransparent,
		},
	})

	// Closing the window (Alt+F4 / WM_CLOSE) hides it instead of quitting: the
	// app stays resident and is summoned via the global hotkey or the tray
	// (ADR-0003).
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		hideWindow()
	})

	// Global hotkey toggles window visibility system-wide (core feature). The
	// combo is configurable via Settings; on failure we fall back to the default.
	toggleHotkey := store.currentSettings().Hotkey
	if toggleHotkey == "" {
		toggleHotkey = defaultHotkey
	}
	registerToggleHotkey(toggleHotkey)
	store.hotkeyReconfigure = reconfigureHotkey
	store.hotkeySuspend = suspendToggleHotkey
	store.hotkeyResume = resumeToggleHotkey
	store.pickDir = func() (string, error) {
		result, err := app.Dialog.OpenFile().
			CanChooseDirectories(true).
			CanChooseFiles(false).
			SetTitle("Select data folder").
			AttachToWindow(mainWindow).
			PromptForSingleSelection()
		return result, err
	}
	store.pickExportDir = func() (string, error) {
		dlg := app.Dialog.OpenFile().
			CanChooseDirectories(true).
			CanChooseFiles(false).
			SetTitle("Select export folder").
			AttachToWindow(mainWindow)
		if dir := userDownloadsDir(); dir != "" {
			dlg = dlg.SetDirectory(dir)
		}
		return dlg.PromptForSingleSelection()
	}
	store.pickOpenPath = func() (string, error) {
		return app.Dialog.OpenFile().
			CanChooseFiles(true).
			CanChooseDirectories(false).
			AddFilter("Markdown / Text", "*.md;*.markdown;*.txt").
			SetTitle("Import markdown").
			AttachToWindow(mainWindow).
			PromptForSingleSelection()
	}

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		setupTray()
		setupPlatformUI()
		registerPlatformHooks()
		if smokeMode {
			go smokeCheck()
			return
		}
		// Wait for the WebView2 to finish loading, then position and show the
		// window in one step: the first visible frame is already at the left
		// edge, so no post-hoc repositioning (no visible jump). On a silent
		// launch (--silent, e.g. auto-start) the window stays hidden and is
		// summoned via hotkey/tray.
		mainWindow.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {
			showMainWindowAtStartup()
		})
		// Safety net for the rare case the navigation event never fires.
		time.AfterFunc(4*time.Second, showMainWindowAtStartup)
	})

	// Defensive: Wails' own setBounds path re-applies LWA_ALPHA=255 for
	// layered windows (e.g. after a DPI-driven resize), which would undo a
	// user-set opacity. Re-apply the persisted value whenever the window
	// is resized or moved by the framework.
	applyOpacityOnWindowChanges()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// onSecondInstanceLaunch handles a second process launch (Windows: named
// mutex + WM_COPYDATA notification carrying the second instance's argv).
func onSecondInstanceLaunch(data application.SecondInstanceData) {
	for _, a := range data.Args {
		switch a {
		case "--quit":
			// The installer asks the running instance to exit gracefully
			// before overwriting the exe: flush pending auto-saves, then
			// quit. Force-killing would risk losing the last keystrokes.
			debugLog("second instance requested quit")
			flushBoundsSave()
			app.Event.Emit("app:quit", "")
			time.Sleep(250 * time.Millisecond)
			app.Quit()
			return
		case "--silent":
			// Auto-start raced with a manual launch: leave the first
			// instance's visibility untouched (a silent start must never pop
			// the window).
			debugLog("second instance was silent; ignoring")
			return
		}
	}
	// A plain second launch means the user started the app again (e.g. from
	// the Start menu): bring the window up, matching the hide/summon model.
	showMainWindow()
}

// toggleHotkeyCallback toggles window visibility and logs the event.
func toggleHotkeyCallback() {
	debugLog("hotkey fired, visible=%v", mainWindow.IsVisible())
	toggleWindow()
}

// registerToggleHotkey binds the given accelerator to the toggle callback.
// A registration failure is logged but does not abort startup.
func registerToggleHotkey(combo string) {
	if err := app.GlobalShortcut.Register(combo, toggleHotkeyCallback); err != nil {
		app.Logger.Error("could not register global shortcut", "error", err)
		debugLog("hotkey register failed: %v", err)
		return
	}
	activeHotkey = combo
	debugLog("hotkey registered: %s", combo)
}

// reconfigureHotkey swaps the toggle hotkey, leaving the previous binding
// untouched on any failure:
//  1. register the new combo (fails fast if invalid / already owned)
//  2. only then unregister the old combo
func reconfigureHotkey(newCombo string) error {
	var err error
	newCombo, err = windowutil.NormalizeHotkey(newCombo)
	if err != nil {
		return err
	}
	if newCombo == activeHotkey {
		return nil
	}
	if err := app.GlobalShortcut.Register(newCombo, toggleHotkeyCallback); err != nil {
		return fmt.Errorf("cannot register %q: %w", newCombo, err)
	}
	old := activeHotkey
	activeHotkey = newCombo
	if old != "" {
		if err := app.GlobalShortcut.Unregister(old); err != nil {
			// Roll back to keep exactly one registered combo.
			_ = app.GlobalShortcut.Unregister(newCombo)
			activeHotkey = ""
			return fmt.Errorf("cannot unregister old hotkey %q: %w", old, err)
		}
	}
	debugLog("hotkey changed: %s -> %s", old, newCombo)
	return nil
}

// suspendToggleHotkey unregisters the active combo (idempotent; no-op when
// nothing is registered). Called before the settings panel starts recording.
func suspendToggleHotkey() error {
	if activeHotkey == "" {
		return nil
	}
	if err := app.GlobalShortcut.Unregister(activeHotkey); err != nil {
		return fmt.Errorf("suspend hotkey %q: %w", activeHotkey, err)
	}
	suspendedHotkey = activeHotkey
	activeHotkey = ""
	debugLog("hotkey suspended: %s", suspendedHotkey)
	return nil
}

// resumeToggleHotkey restores the toggle hotkey after a suspension. If a new
// combo was registered while suspended (reconfigureHotkey ran), it is already
// active and there is nothing to restore.
func resumeToggleHotkey() error {
	if activeHotkey != "" {
		// A new combo is already registered; drop the stale suspension.
		suspendedHotkey = ""
		return nil
	}
	if suspendedHotkey == "" {
		return nil
	}
	if err := app.GlobalShortcut.Register(suspendedHotkey, toggleHotkeyCallback); err != nil {
		return fmt.Errorf("resume hotkey %q: %w", suspendedHotkey, err)
	}
	activeHotkey = suspendedHotkey
	suspendedHotkey = ""
	debugLog("hotkey resumed: %s", activeHotkey)
	return nil
}

// userDownloadsDir is provided by the platform layer (Win32 known-folder on
// Windows, ~/Downloads on macOS).

// positionWindowAtStartup restores the window to its last saved bounds
// (Settings.Window*) when those still sit on a visible screen, otherwise
// falls back to the default: left edge of the primary screen's work area,
// full height, one third of the screen wide.
//
// It uses a raw Win32 SetWindowPos instead of WebviewWindow.SetPosition:
// the Wails call goes through InvokeSync + setBounds and is unreliable for a
// still-hidden window (the window ends up centered), and its setBounds path
// also re-applies LWA_ALPHA=255 which would clobber the window opacity.
func positionWindowAtStartup() {
	st := store.currentSettings()
	if st.WindowWidth > 0 && st.WindowHeight > 0 &&
		boundsVisible(st.WindowX, st.WindowY, st.WindowWidth, st.WindowHeight) {
		w := max(st.WindowWidth, minWidth)
		setWindowBounds(st.WindowX, st.WindowY, w, max(st.WindowHeight, minHeight))
		return
	}
	screen := app.Screen.GetPrimary()
	if screen == nil {
		mainWindow.Center()
		return
	}
	pwa := screen.PhysicalWorkArea
	width := pwa.Width / 3
	if width < minWidth {
		width = minWidth
	}
	setWindowBounds(pwa.X, pwa.Y, width, pwa.Height)
}

// boundsVisible reports whether the given rect (physical pixels) overlaps any
// screen's physical work area, so a window left on a disconnected monitor
// falls back to the default placement instead of reopening off-screen. Pure
// geometry lives in internal/windowutil (RectOverlapsAny) for unit-testing.
func boundsVisible(x, y, w, h int) bool {
	all := app.Screen.GetAll()
	screens := make([]windowutil.ScreenArea, 0, len(all))
	for _, s := range all {
		wa := s.PhysicalWorkArea
		screens = append(screens, windowutil.ScreenArea{X: wa.X, Y: wa.Y, Width: wa.Width, Height: wa.Height})
	}
	return windowutil.RectOverlapsAny(x, y, w, h, screens)
}

// setWindowBounds moves/resizes the native window. Implemented per platform
// (Win32 SetWindowPos on Windows; logical-point conversion on macOS).

// positionedOnce guards the startup placement so later Show() calls (tray /
// hotkey) do not yank the window back to the left edge.
var positionedOnce = false

// silentStart is true when launched with --silent (auto-start): the window
// is positioned but never shown; the user summons it via hotkey or tray.
var silentStart = false

// smokeMode is true when launched with --smoke (CI launch smoke test): the
// app boots, creates the window, positions and shows it, reports SMOKE OK and
// exits 0 (or exits 1 on any failure). This is the only place the native
// startup path is verified without a real desktop session.
var smokeMode = false

// showMainWindow makes the window visible and focused. It emits app:show so
// the frontend can move the caret to the end of the note, ready to type — a
// sticky note is summoned to be written in. Plain refocuses (the window stays
// visible, e.g. the user alt-tabs back) do NOT emit this event, so the
// frontend's caret memory / selection is never disturbed.
func showMainWindow() {
	app.Event.Emit("app:show", "")
	mainWindow.Show()
	mainWindow.Focus()
}

// showMainWindowAtStartup positions the window once the WebView2 has settled
// and shows it (unless this is a silent launch). Runs at most once.
func showMainWindowAtStartup() {
	if positionedOnce {
		return
	}
	positionedOnce = true
	positionWindowAtStartup()
	applyWindowOpacity()
	if !silentStart {
		showMainWindow()
	}
}

// applyOpacityOnWindowChanges re-applies the window opacity after framework
// resizes/moves (on Windows, Wails' setBounds re-applies LWA_ALPHA=255 for
// layered windows, undoing a user-set transparency; on macOS the native
// opacity call is a no-op) and debounce-saves the window bounds so the
// position/size survive a restart.
func applyOpacityOnWindowChanges() {
	onChange := func(*application.WindowEvent) {
		applyWindowOpacity()
		scheduleBoundsSave()
	}
	mainWindow.OnWindowEvent(events.Common.WindowDidResize, onChange)
	mainWindow.OnWindowEvent(events.Common.WindowDidMove, onChange)
}

// --- window bounds persistence (debounced save) ---

// boundsSaveDelay coalesces the move/resize event storm during a drag into a
// single settings write.
const boundsSaveDelay = 400 * time.Millisecond

// boundsSaveTimer is the pending debounce; nil when no save is scheduled.
var boundsSaveTimer *time.Timer

// scheduleBoundsSave debounces a window-bounds save. The startup placement
// also fires move/resize events; saving those is harmless (they are the
// window's current state) but the 400ms delay keeps drags to one write.
func scheduleBoundsSave() {
	if boundsSaveTimer != nil {
		boundsSaveTimer.Stop()
	}
	boundsSaveTimer = time.AfterFunc(boundsSaveDelay, saveWindowBoundsNow)
}

// flushBoundsSave persists any pending bounds immediately (called before the
// window hides or the app quits so the last drag position is not lost).
func flushBoundsSave() {
	if boundsSaveTimer == nil {
		return
	}
	boundsSaveTimer.Stop()
	boundsSaveTimer = nil
	saveWindowBoundsNow()
}

// saveWindowBoundsNow reads the window bounds (physical pixels) and persists
// them. Implemented per platform (Win32 GetWindowRect / macOS Position+Size
// × scale factor). Best effort: a failed write only costs the last drag.

// setupTray creates the system tray icon with a menu (Show/Hide, Quit). Left
// click toggles window visibility; the menu mirrors the same actions.
func setupTray() {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Slite Note")
	tray.SetLabel("Slite Note")

	menu := app.NewMenu()
	menu.Add("Show/Hide").OnClick(func(ctx *application.Context) {
		toggleWindow()
	})
	menu.Add("Settings...").OnClick(func(ctx *application.Context) {
		app.Event.Emit("app:open-settings", "")
		if !mainWindow.IsVisible() {
			mainWindow.Show()
		}
		mainWindow.Focus()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(ctx *application.Context) {
		// Give the frontend a moment to flush pending auto-saves.
		flushBoundsSave()
		app.Event.Emit("app:quit", "")
		time.Sleep(250 * time.Millisecond)
		app.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(toggleWindow)
	// NOTE: do NOT call tray.Run() here — SystemTray.New() already runs the
	// tray when the app is running (runOrDeferToAppRun), and a second Run()
	// re-adds the icon (ShellNotifyIcon NIM_ADD) producing a duplicate tray
	// icon.
}

// smokeCheck runs the CI launch smoke test: give the window a moment to
// finish creating, then verify it exists and is visible (unless --silent was
// also passed), print SMOKE OK and exit 0. Any failure exits 1 so the CI job
// fails. Uses the same startup path as a normal launch.
func smokeCheck() {
	time.Sleep(1500 * time.Millisecond)
	if mainWindow == nil {
		log.Println("SMOKE FAIL: main window was never created")
		os.Exit(1)
	}
	positionWindowAtStartup()
	applyWindowOpacity()
	if !silentStart {
		showMainWindow()
		// Window visibility is async on macOS: IsVisible() maps to
		// NSWindow.occlusionState, which WindowServer only flips after it
		// composites the first frame (CI runners have no real display session,
		// so the delay is variable). Poll instead of a fixed 200ms sleep so a
		// slow-but-healthy window does not fail the smoke test; the deadline
		// still catches a window that genuinely never appears.
		deadline := time.Now().Add(5 * time.Second)
		for !mainWindow.IsVisible() {
			if time.Now().After(deadline) {
				log.Println("SMOKE FAIL: window not visible after show")
				os.Exit(1)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	debugLog("smoke ok")
	fmt.Println("SMOKE OK")
	app.Quit()
	os.Exit(0)
}

// toggleWindow shows or hides the main window (hotkey, tray click, tray menu).
func toggleWindow() {
	if mainWindow.IsVisible() {
		hideWindow()
	} else {
		showMainWindow()
	}
}

func hideWindow() {
	// Let the frontend flush pending auto-saves before the window disappears.
	app.Event.Emit("app:hide", "")
	flushBoundsSave()
	mainWindow.Hide()
}
