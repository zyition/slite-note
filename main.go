package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/windows"
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
// while the app is headless).
func debugLog(format string, args ...any) {
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
	for _, a := range os.Args {
		if a == "--silent" {
			silentStart = true
			break
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
		Title:            "slite-note",
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
	newCombo = strings.TrimSpace(newCombo)
	if newCombo == "" {
		return fmt.Errorf("hotkey must not be empty")
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

// userDownloadsDir returns the user's Downloads folder via the known-folder
// API (honours a redirected Downloads location), falling back to
// <home>/Downloads, then "" (no default directory) if neither resolves.
func userDownloadsDir() string {
	if d, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0); err == nil && d != "" {
		return d
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Downloads")
	}
	return ""
}

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
// falls back to the default placement instead of reopening off-screen.
func boundsVisible(x, y, w, h int) bool {
	for _, s := range app.Screen.GetAll() {
		wa := s.PhysicalWorkArea
		ox := max(x, wa.X)
		oy := max(y, wa.Y)
		ox2 := min(x+w, wa.X+wa.Width)
		oy2 := min(y+h, wa.Y+wa.Height)
		if ox2 > ox && oy2 > oy {
			return true
		}
	}
	return false
}

// setWindowBounds moves/resizes the native window via SetWindowPos.
func setWindowBounds(x, y, w, h int) {
	if mainWindow == nil {
		return
	}
	hwnd := uintptr(mainWindow.NativeWindow())
	if hwnd == 0 {
		return
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(swpNoZorder|swpNoActivate))
}

// positionedOnce guards the startup placement so later Show() calls (tray /
// hotkey) do not yank the window back to the left edge.
var positionedOnce = false

// silentStart is true when launched with --silent (auto-start): the window
// is positioned but never shown; the user summons it via hotkey or tray.
var silentStart = false

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

// --- window opacity (Win32 WS_EX_LAYERED + SetLayeredWindowAttributes) ---
//
// Wails v3 has no opacity API, so we drive the window through user32 directly.
// alpha must be in (0, 1]; values below 0.05 remove the layered style (fully
// opaque). The whole window — WebView2 content included — is affected.

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	procSetLayeredAttrs = user32.NewProc("SetLayeredWindowAttributes")
	procGetWindowLong   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLong   = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
)

const (
	gwlExStyle    = int(-20)
	wsExLayered   = 0x00080000
	lwaAlpha      = 0x00000002
	opacityMin    = 0.05
	opacityFloor  = 0.3 // UI slider minimum; anything below means "not set"
	swpNoZorder   = 0x0004
	swpNoActivate = 0x0010
)

// gwlExStylePtr is the runtime-converted nIndex (GWLP_EXSTYLE = -20) for
// GetWindowLongPtrW/SetWindowLongPtrW (uintptr rejects negative constants).
var (
	gwlExStyleVar int = -20
	gwlExStylePtr     = uintptr(gwlExStyleVar)
)

// setWindowOpacity applies a whole-window alpha (1 = fully opaque). A value of
// 0 or >= 1 removes the layered style entirely to avoid any DWM side effects.
func setWindowOpacity(alpha float64) error {
	if mainWindow == nil {
		return nil
	}
	hwnd := uintptr(mainWindow.NativeWindow())
	if hwnd == 0 {
		return nil
	}
	exStyle, _, _ := procGetWindowLong.Call(hwnd, gwlExStylePtr)
	if alpha <= 0 || alpha >= 1 {
		if exStyle&wsExLayered != 0 {
			procSetWindowLong.Call(hwnd, gwlExStylePtr, exStyle&^wsExLayered)
		}
		return nil
	}
	if alpha < opacityMin {
		alpha = opacityMin
	}
	procSetWindowLong.Call(hwnd, gwlExStylePtr, exStyle|wsExLayered)
	_, _, err := procSetLayeredAttrs.Call(hwnd, 0, uintptr(byte(alpha*255+0.5)), lwaAlpha)
	if err != nil && err != windows.ERROR_SUCCESS {
		return err
	}
	return nil
}

// setOpacityOverride lifts the window to fully opaque while a modal overlay
// is open and suppresses the persisted opacity until released. Releasing
// restores whatever the user last set.
func setOpacityOverride(on bool) {
	opacityOverride = on
	if on {
		if err := setWindowOpacity(1); err != nil {
			debugLog("set opacity (override) failed: %v", err)
		}
	} else {
		applyWindowOpacity()
	}
}

// applyWindowOpacity applies the persisted opacity (defaulting to opaque when
// unset). Called at startup and from window-change hooks.
func applyWindowOpacity() {
	if opacityOverride {
		_ = setWindowOpacity(1)
		return
	}
	op := store.currentSettings().Opacity
	if op < opacityFloor || op > 1 {
		op = 1 // unset or out of range: fully opaque
	}
	if err := setWindowOpacity(op); err != nil {
		debugLog("set opacity failed: %v", err)
	}
}

// applyOpacityOnWindowChanges re-applies the window opacity after framework
// resizes/moves, because Wails' setBounds re-applies LWA_ALPHA=255 for
// layered windows (which would undo a user-set transparency), and debounce-
// saves the window bounds so the position/size survive a restart.
func applyOpacityOnWindowChanges() {
	onChange := func(*application.WindowEvent) {
		applyWindowOpacity()
		scheduleBoundsSave()
	}
	mainWindow.OnWindowEvent(events.Common.WindowDidResize, onChange)
	mainWindow.OnWindowEvent(events.Common.WindowDidMove, onChange)
}

// --- window bounds persistence (Win32 GetWindowRect + debounced save) ---

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

// saveWindowBoundsNow reads the window rect (physical pixels, matching the
// SetWindowPos-based placement) and persists it. Best effort: a failed write
// only costs the last drag position.
func saveWindowBoundsNow() {
	boundsSaveTimer = nil
	if mainWindow == nil {
		return
	}
	hwnd := uintptr(mainWindow.NativeWindow())
	if hwnd == 0 {
		return
	}
	var r winRect
	if _, _, err := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); err != nil && err != windows.ERROR_SUCCESS {
		debugLog("GetWindowRect failed: %v", err)
		return
	}
	w := int(r.Right - r.Left)
	h := int(r.Bottom - r.Top)
	if w <= 0 || h <= 0 {
		return
	}
	if err := store.SaveWindowBounds(int(r.Left), int(r.Top), w, h); err != nil {
		debugLog("save window bounds failed: %v", err)
	}
}

// winRect mirrors the Win32 RECT layout (LONG left/top/right/bottom).
type winRect struct {
	Left, Top, Right, Bottom int32
}

// setupTray creates the system tray icon with a menu (Show/Hide, Quit). Left
// click toggles window visibility; the menu mirrors the same actions.
func setupTray() {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("slite-note")
	tray.SetLabel("slite-note")

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
