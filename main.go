package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

// droppedFilesEvent tells the frontend where a native (macOS) file drop
// landed. Coordinates are CSS pixels in the window's content area — the same
// space DOM drag events report, so the editor can reuse its placement lookup.
type droppedFilesEvent struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func init() {
	application.RegisterEvent[string]("app:hide")
	application.RegisterEvent[string]("app:show")
	application.RegisterEvent[string]("app:quit")
	application.RegisterEvent[string]("app:flush-before-destroy")
	application.RegisterEvent[string]("app:restore-view-state")
	application.RegisterEvent[droppedFilesEvent]("app:files-dropped")
}

const (
	mainWindowName = "main"
	defaultHotkey  = "Alt+Shift+S"
	initialWidth   = 480 // startup width; adjusted to 1/3 of the screen in positionWindowAtStartup
	minWidth       = 320
	minHeight      = 320

	// revealFallbackDelay bounds how long a recreated window stays hidden while
	// waiting for the frontend's ready signal; after it, show it anyway.
	revealFallbackDelay = 3 * time.Second
	// destroyFlushTimeout bounds the save handshake before a lightweight-mode
	// window teardown.
	destroyFlushTimeout = 3 * time.Second
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

// startupBackgroundColour resolves the window-creation background from the
// persisted theme so the very first frame already matches the UI — a
// dark-theme user must never see the historical note-yellow flash when the
// window (re)presents before the webview does. RGB values mirror THEMES in
// frontend/src/services/theme.ts; keep them in sync.
func startupBackgroundColour(theme string) application.RGBA {
	dark := application.NewRGB(31, 31, 31)
	switch theme {
	case "dark":
		return dark
	case "gray":
		return application.NewRGB(244, 244, 242)
	case "yellow":
		return application.NewRGB(255, 243, 176) // legacy note yellow
	default: // "system" or unknown
		if systemPrefersDark() {
			return dark
		}
		return application.NewRGB(255, 243, 176)
	}
}

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

// attachmentMiddleware routes /attachments/* to the data directory instead of
// the embedded AssetServer. Blocks reference media as the relative path
// "attachments/<hash>.<ext>"; the frontend resolves that to /attachments/...,
// so a data-dir migration never forces a rewrite of block content.
func attachmentMiddleware(store *Store) application.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/attachments/") {
				serveAttachment(store, w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// serveAttachment writes a single attachment file from the active data
// directory. Only the final path segment is used (path.Base) so ../ sequences
// or nested paths can never escape the attachments directory.
func serveAttachment(store *Store, w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.URL.Path)
	if name == "." || name == "" {
		http.NotFound(w, r)
		return
	}
	// no-store, deliberately: attachments are content-addressed files on local
	// disk, so refetching on every render costs nothing — while a poisoned
	// cache entry sticks forever. A response truncated mid-load (e.g. the
	// window hiding while a picture was still loading) once got cached by
	// WKWebView under its heuristic caching, and the image then stayed broken
	// in the app even though the bytes on disk were fine and Safari rendered
	// the same file correctly. With no-store nothing is ever stored, so a bad
	// response cannot outlive the request that produced it.
	w.Header().Set("Cache-Control", "no-store")
	file := filepath.Join(store.currentDataDir(), "attachments", name)
	http.ServeFile(w, r, file)
}

var (
	app   *application.App
	store *Store
	tray  *application.SystemTray
)

// --- window lifecycle / lightweight mode ---
//
// The app owns at most one WebView window. Hiding keeps it warm so summoning
// is instant; after Settings.LightweightModeMinutes hidden minutes the window
// is closed, which tears down WebView2/WKWebView and the renderer processes,
// and the next summon recreates it.
//
// windowStateMu guards all fields below. The main window itself is looked up
// through getMainWindow(): a closed window is removed from Wails' manager, so
// the lookup is nil exactly when there is no window to talk to.
var (
	windowStateMu sync.Mutex

	// destroyWindowID is the window a deliberate Close() is meant to destroy.
	// The WindowClosing hook lets that close through and turns every other
	// close request into a hide.
	destroyWindowID uint

	// destroying is true between requesting that close and the window leaving
	// the manager; show requests during that gap are replayed afterwards.
	destroying         bool
	revealAfterDestroy bool

	// quitting suppresses hide-on-close and timer-driven recreation while the
	// app is terminating.
	quitting bool

	// pendingReveal is set while a recreated window waits for both navigation
	// completion and the frontend ready signal before it is shown.
	pendingReveal  bool
	frontendReady  bool
	navigationDone bool

	// pendingOpenSettings replays the macOS Settings… menu action once a
	// recreated window's frontend is ready.
	pendingOpenSettings bool

	lightweightTimer *time.Timer
)

// getMainWindow returns the live main window, or nil once it has been
// destroyed. Never cache the returned pointer across a possible close.
func getMainWindow() *application.WebviewWindow {
	if app == nil {
		return nil
	}
	w, ok := app.Window.Get(mainWindowName)
	if !ok || w == nil {
		return nil
	}
	ww, ok := w.(*application.WebviewWindow)
	if !ok {
		return nil
	}
	return ww
}

func setDestroyTarget(id uint) {
	windowStateMu.Lock()
	destroyWindowID = id
	windowStateMu.Unlock()
}

func isDestroyTarget(id uint) bool {
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	return quitting || destroyWindowID == id
}

func setQuitting() {
	windowStateMu.Lock()
	quitting = true
	windowStateMu.Unlock()
	cancelLightweightTimer()
}

func isQuitting() bool {
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	return quitting
}

func setDestroying(on bool) {
	windowStateMu.Lock()
	destroying = on
	if !on {
		revealAfterDestroy = false
	}
	windowStateMu.Unlock()
}

func isDestroying() bool {
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	return destroying
}

func setPendingReveal(on bool) {
	windowStateMu.Lock()
	pendingReveal = on
	windowStateMu.Unlock()
}

func isPendingReveal() bool {
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	return pendingReveal
}

func markFrontendReady() {
	windowStateMu.Lock()
	frontendReady = true
	windowStateMu.Unlock()
	maybeRevealRecreatedWindow()
}

func setFrontendReady(ready bool) {
	windowStateMu.Lock()
	frontendReady = ready
	windowStateMu.Unlock()
}

func isFrontendReady() bool {
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	return frontendReady
}

func markNavigationDone() {
	windowStateMu.Lock()
	navigationDone = true
	windowStateMu.Unlock()
	maybeRevealRecreatedWindow()
}

func markPendingOpenSettings() {
	windowStateMu.Lock()
	pendingOpenSettings = true
	windowStateMu.Unlock()
}

func takePendingOpenSettings() bool {
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	pending := pendingOpenSettings
	pendingOpenSettings = false
	return pending
}

// maybeRevealRecreatedWindow shows a recreated window once its page has
// navigated and the frontend has attached its listeners. Until then the window
// stays hidden, so a summon never flashes an empty page.
func maybeRevealRecreatedWindow() {
	windowStateMu.Lock()
	ready := pendingReveal && frontendReady && navigationDone
	if ready {
		pendingReveal = false
	}
	windowStateMu.Unlock()
	if !ready {
		return
	}
	revealMainWindow()
}

// revealMainWindow makes a freshly created window visible, restoring its
// place/opacity first and telling the frontend it was summoned.
func revealMainWindow() {
	w := getMainWindow()
	if w == nil {
		return
	}
	markStartupPositioned()
	positionWindowAtStartup()
	applyWindowOpacity()
	w.Show()
	w.Focus()
	app.Event.Emit("app:show", "")
	if state := takePendingViewState(); state != "" {
		debugLog("restore: sending view state")
		app.Event.Emit("app:restore-view-state", state)
	}
	if takePendingOpenSettings() {
		app.Event.Emit("app:open-settings", "")
	}
}

// --- caret/selection and scroll handoff (memory only) ---

// pendingViewState is the editor state captured before a lightweight-mode
// teardown and replayed after the next recreation. It is deliberately not
// persisted: quitting the process forgets it.
var (
	viewStateMu      sync.Mutex
	pendingViewState string
)

func setPendingViewState(state string) {
	viewStateMu.Lock()
	pendingViewState = state
	viewStateMu.Unlock()
}

func takePendingViewState() string {
	viewStateMu.Lock()
	state := pendingViewState
	pendingViewState = ""
	viewStateMu.Unlock()
	return state
}

// --- save handshake before lightweight teardown ---

var (
	flushMu    sync.Mutex
	flushToken string
	flushDone  chan struct{}
)

func beginFrontendFlush() (string, <-chan struct{}) {
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	done := make(chan struct{})
	flushMu.Lock()
	flushToken, flushDone = token, done
	flushMu.Unlock()
	return token, done
}

// ackFrontendFlush records the view state (when the page sent one) before
// releasing the waiting teardown, so the state cannot be lost to a race with
// the window close.
func ackFrontendFlush(token, viewState string) bool {
	flushMu.Lock()
	defer flushMu.Unlock()
	if flushToken != token || flushDone == nil {
		return false
	}
	if viewState != "" {
		setPendingViewState(viewState)
	}
	close(flushDone)
	flushDone = nil
	return true
}

func abandonFrontendFlush() {
	flushMu.Lock()
	flushToken, flushDone = "", nil
	flushMu.Unlock()
}

// waitForFrontendFlush asks the page to persist pending edits and waits,
// briefly, for its acknowledgement. A missing/blocked frontend must not keep
// the window (and its renderer memory) alive forever.
func waitForFrontendFlush() {
	if !isFrontendReady() || app == nil {
		return
	}
	token, done := beginFrontendFlush()
	app.Event.Emit("app:flush-before-destroy", token)
	select {
	case <-done:
	case <-time.After(destroyFlushTimeout):
		abandonFrontendFlush()
		debugLog("lightweight: frontend flush timed out")
	}
}

// --- lightweight timer ---

// syncLightweightTimer re-evaluates the timer after a settings change.
func syncLightweightTimer() {
	if app == nil {
		return
	}
	if getMainWindow() == nil {
		cancelLightweightTimer()
		return
	}
	if w := getMainWindow(); w != nil && w.IsVisible() {
		cancelLightweightTimer()
		return
	}
	scheduleLightweightDestroy()
}

// scheduleLightweightDestroy (re)arms the hidden-window teardown. A value of 0
// cancels it, so the default keeps the previous always-resident behaviour.
func scheduleLightweightDestroy() {
	minutes := store.currentSettings().LightweightModeMinutes
	windowStateMu.Lock()
	if lightweightTimer != nil {
		lightweightTimer.Stop()
		lightweightTimer = nil
	}
	if minutes <= 0 || quitting {
		windowStateMu.Unlock()
		return
	}
	lightweightTimer = time.AfterFunc(time.Duration(minutes)*time.Minute, destroyHiddenMainWindow)
	windowStateMu.Unlock()
	debugLog("lightweight: destroy scheduled in %d min", minutes)
}

func cancelLightweightTimer() {
	windowStateMu.Lock()
	if lightweightTimer != nil {
		lightweightTimer.Stop()
		lightweightTimer = nil
	}
	windowStateMu.Unlock()
}

func destroyHiddenMainWindow() {
	windowStateMu.Lock()
	lightweightTimer = nil
	windowStateMu.Unlock()

	if isQuitting() || isPendingReveal() || isDestroying() {
		return
	}
	w := getMainWindow()
	if w == nil || w.IsVisible() {
		return
	}
	windowID := w.ID()
	debugLog("lightweight: destroying hidden main window")
	waitForFrontendFlush()

	// The user may have summoned it during the flush handshake. Mark the
	// teardown in progress before the final visibility check: a summon from
	// this point on is replayed after the window has gone.
	setDestroying(true)
	if isQuitting() {
		setDestroying(false)
		return
	}
	w = getMainWindow()
	if w == nil || w.ID() != windowID || w.IsVisible() {
		setDestroying(false)
		return
	}
	setFrontendReady(false)
	flushBoundsSave()
	setDestroyTarget(windowID)
	w.Close()

	// Close() queues the WindowClosing event; clear the destroying state only
	// after the window has actually left the manager.
	go func(id uint) {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if cur := getMainWindow(); cur == nil || cur.ID() != id {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		windowStateMu.Lock()
		destroying = false
		reveal := revealAfterDestroy
		revealAfterDestroy = false
		windowStateMu.Unlock()
		if reveal && !isQuitting() {
			showMainWindow()
		}
	}(windowID)
}

// onMainWindowHidden schedules the lightweight teardown no matter which path
// hid the window (Go hideWindow or the frontend's Wails Window.Hide call).
func onMainWindowHidden() {
	trimWorkingSetAfterHide()
	if isPendingReveal() || isDestroying() || isQuitting() {
		return
	}
	if w := getMainWindow(); w != nil && !w.IsVisible() {
		scheduleLightweightDestroy()
	}
}

func onMainWindowShown() {
	cancelLightweightTimer()
}

// trayMenuStrings holds the native tray-menu labels per supported UI
// language. English is the fallback: the tray is built before the webview
// loads, and "" (follow the OS) and unknown values can only be resolved by
// the frontend, which pushes the resolved locale via SetTrayLanguage.
type trayMenuLabels struct{ showHide, quit string }

var trayMenuStrings = map[string]trayMenuLabels{
	"en":    {"Show/Hide", "Quit"},
	"zh-CN": {"显示/隐藏", "退出"},
}

func trayLabelsFor(language string) trayMenuLabels {
	if labels, ok := trayMenuStrings[language]; ok {
		return labels
	}
	return trayMenuStrings["en"]
}

// currentTrayLanguage tracks the language the tray menu was last built with,
// so repeat pushes of the same locale do not rebuild the native menu.
var currentTrayLanguage string

// applyTrayLanguage rebuilds the tray menu with the labels for language.
// Safe to call before setupTray (no-op) or repeatedly with the same value.
func applyTrayLanguage(language string) {
	if tray == nil || language == currentTrayLanguage {
		return
	}
	currentTrayLanguage = language
	tray.SetMenu(buildTrayMenu(language))
}

func buildTrayMenu(language string) *application.Menu {
	labels := trayLabelsFor(language)
	menu := app.NewMenu()
	menu.Add(labels.showHide).OnClick(func(ctx *application.Context) {
		toggleWindow()
	})
	menu.AddSeparator()
	menu.Add(labels.quit).OnClick(func(ctx *application.Context) {
		setQuitting()
		// Give the frontend a moment to flush pending auto-saves. When
		// lightweight mode already destroyed the window there is no page to
		// flush and quitting can be immediate.
		flushBoundsSave()
		if getMainWindow() != nil {
			app.Event.Emit("app:quit", "")
			time.Sleep(250 * time.Millisecond)
		}
		app.Quit()
	})
	return menu
}

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

	// Startup orphan-attachment cleanup: only when enabled in settings. Runs in
	// a goroutine so it never blocks the first paint, and only touches files
	// inside attachments/ (the diff against every note's references) — a blob
	// shared by two notes survives deleting one of them.
	if store.currentSettings().AutoCleanAttachments {
		go func() {
			if n, err := store.CleanOrphanAttachments(); err == nil && n > 0 {
				debugLog("slite: cleaned %d orphan attachment(s) at startup", n)
			}
		}()
	}

	app = application.New(application.Options{
		Name:        "slite-note",
		Description: "A minimal desktop sticky notes app",
		Services: []application.Service{
			application.NewService(store),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// The default handler only serves the embedded frontend. Route
			// /attachments/* to the data directory so pasted/uploaded media
			// (referenced as relative "attachments/<hash>.<ext>" in blocks, then
			// resolved to /attachments/<hash>.<ext> on the frontend) is served
			// from disk with full Range/Cache support (see attachmentMiddleware).
			Middleware: attachmentMiddleware(store),
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
			// The window is destroyed in lightweight mode; the app must stay
			// resident (tray + hotkey) instead of quitting with its last window.
			DisableQuitOnLastWindowClosed: true,
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
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// A silent launch (auto-start) needs no UI and therefore no WebView at
	// all: the window is created on the first hotkey/tray/second-instance
	// summon. This is independent of lightweight mode and saves the whole
	// WebView2/WKWebView tree until the user actually asks for the note.
	// --smoke still creates one so the CI launch check has something to verify.
	if !silentStart || smokeMode {
		createMainWindow()
	}

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
		dlg := app.Dialog.OpenFile().
			CanChooseDirectories(true).
			CanChooseFiles(false).
			SetTitle("Select data folder")
		if w := getMainWindow(); w != nil {
			dlg = dlg.AttachToWindow(w)
		}
		return dlg.PromptForSingleSelection()
	}
	store.pickExportDir = func() (string, error) {
		dlg := app.Dialog.OpenFile().
			CanChooseDirectories(true).
			CanChooseFiles(false).
			SetTitle("Select export folder")
		if w := getMainWindow(); w != nil {
			dlg = dlg.AttachToWindow(w)
		}
		if dir := userDownloadsDir(); dir != "" {
			dlg = dlg.SetDirectory(dir)
		}
		return dlg.PromptForSingleSelection()
	}
	store.pickOpenPath = func() (string, error) {
		dlg := app.Dialog.OpenFile().
			CanChooseFiles(true).
			CanChooseDirectories(false).
			AddFilter("Markdown / Text", "*.md;*.markdown;*.txt").
			SetTitle("Import markdown")
		if w := getMainWindow(); w != nil {
			dlg = dlg.AttachToWindow(w)
		}
		return dlg.PromptForSingleSelection()
	}

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		setupTray()
		setupPlatformUI()
		registerPlatformHooks()
		if smokeMode {
			go smokeCheck()
			return
		}
		// Safety net for the rare case the navigation event never fires. The
		// normal path is handled per window in setupMainWindow. A silent start
		// has no window to place; the first summon handles its own reveal.
		if getMainWindow() != nil {
			time.AfterFunc(4*time.Second, showMainWindowAtStartup)
		}
	})

	app.OnShutdown(func() {
		setQuitting()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// mainWindowOptions is shared by the first window and every recreation in
// lightweight mode, so both look and behave identically.
func mainWindowOptions() application.WebviewWindowOptions {
	settings := store.currentSettings()
	return application.WebviewWindowOptions{
		Name:        mainWindowName,
		Title:       "Slite Note",
		Width:       initialWidth,
		Height:      480,
		MinWidth:    280,
		MinHeight:   320,
		Frameless:   true,
		AlwaysOnTop: settings.AlwaysOnTop,
		// Resolve from the persisted theme instead of hardcoding note yellow:
		// the first presented frame (and any repaint before the webview
		// presents again, e.g. right after a summon) must not flash yellow
		// for a dark-theme user. The frontend keeps it in sync afterwards
		// (bridge.setWindowBackground). See startupBackgroundColour.
		BackgroundColour: startupBackgroundColour(settings.Theme),
		URL:              "/",
		// Start hidden; the startup path or revealMainWindow shows it once the
		// page is ready, so the first visible frame is already in place.
		Hidden: true,
		// EnableFileDrop is required for HTML5 file drops to reach the frontend at
		// all. Wails' runtime listens for file drags on documentElement and, while
		// this flag is false, sets `dataTransfer.dropEffect = "none"` on every
		// dragover — the browser then never fires the `drop` event, so BlockNote's
		// own drop handler can never see the file. With the flag on, the runtime
		// reports `copy` only while the cursor is over an element carrying the
		// `data-file-drop-target` attribute (see the editor wrapper in App.tsx);
		// everything else still shows the "no drop" cursor. On Windows the editor
		// reads the dropped files straight from the DOM event; on macOS the drop
		// is intercepted natively and arrives via the WindowFilesDropped event
		// below.
		EnableFileDrop: true,
		// macOS: make the window shell transparent so the frontend's semi-
		// transparent note background (--bg-opacity) shows the desktop through.
		// Ignored on Windows (which uses WS_EX_LAYERED for opacity instead).
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTransparent,
		},
	}
}

// createMainWindow builds the window and wires its lifecycle. Called directly
// before App.Run and, in lightweight mode, again on summon.
func createMainWindow() *application.WebviewWindow {
	windowStateMu.Lock()
	frontendReady = false
	navigationDone = false
	windowStateMu.Unlock()

	w := app.Window.NewWithOptions(mainWindowOptions())
	setupMainWindow(w)
	return w
}

// createMainWindowOnMainThread marshals window creation for callers on the
// hotkey/tray goroutines; Wails' UI creation must happen on the main thread.
func createMainWindowOnMainThread() *application.WebviewWindow {
	var w *application.WebviewWindow
	application.InvokeSync(func() {
		w = createMainWindow()
	})
	return w
}

func setupMainWindow(w *application.WebviewWindow) {
	// Close = hide (ADR-0003). Hooks run before Wails' built-in close listener
	// and can cancel it; a deliberate lightweight teardown marks the window ID
	// so that one Close() goes through and removes the native window.
	w.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if isDestroyTarget(w.ID()) {
			return
		}
		event.Cancel()
		hideWindow()
	})
	// A no-op listener keeps the native close event subscribed on macOS/Linux,
	// where Wails only registers platform callbacks for event IDs that have a
	// listener. The hook above still decides whether the close happens.
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {})

	// Track visibility regardless of which path hid/shown the window: the
	// frontend calls Wails Window.Hide() directly from its title bar, which
	// never passes through Go's hideWindow.
	w.OnWindowEvent(events.Common.WindowHide, func(*application.WindowEvent) {
		onMainWindowHidden()
	})
	w.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		onMainWindowShown()
	})

	// macOS: file drops are intercepted by Wails' native drag overlay (see
	// EnableFileDrop above), so the DOM `drop` event never fires with files and
	// the editor cannot see them. The runtime reports the drop here instead,
	// with the file paths and the content-view coordinates of the release point
	// (Cocoa points — 1 pt = 1 CSS px in the WKWebView, so they are directly
	// comparable with the Windows DOM path's clientX/clientY). The image files
	// are recorded in the store for the frontend to fetch (LoadDroppedImages)
	// and the release point is broadcast so the picture lands where the pointer
	// was. Windows keeps the plain DOM path: the event fires there too (after
	// the DOM drop has already inserted the image), hence the darwin gate.
	w.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		if runtime.GOOS != "darwin" {
			return
		}
		paths := filterDroppedImages(event.Context().DroppedFiles())
		if len(paths) == 0 {
			return
		}
		x, y := 0, 0
		if details := event.Context().DropTargetDetails(); details != nil {
			x, y = details.X, details.Y
		}
		store.noteDroppedImages(paths)
		app.Event.Emit("app:files-dropped", droppedFilesEvent{X: x, Y: y})
	})

	// Defensive: Wails' own setBounds path re-applies LWA_ALPHA=255 for
	// layered windows (e.g. after a DPI-driven resize), which would undo a
	// user-set opacity. Re-apply the persisted value whenever the window
	// is resized or moved by the framework.
	onChange := func(*application.WindowEvent) {
		applyWindowOpacity()
		scheduleBoundsSave()
	}
	w.OnWindowEvent(events.Common.WindowDidResize, onChange)
	w.OnWindowEvent(events.Common.WindowDidMove, onChange)

	// Wait for the WebView to finish loading, then position and show it in one
	// step: the first visible frame is already at the left edge, so no
	// post-hoc repositioning (no visible jump). The navigation-completed event
	// is platform-specific: Windows emits
	// events.Windows.WebViewNavigationCompleted, macOS emits
	// events.Mac.WebViewDidFinishNavigation.
	w.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(*application.WindowEvent) {
		onMainWindowNavigation()
	})
	w.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
		onMainWindowNavigation()
	})
}

// onMainWindowNavigation completes the first-boot show or, for a recreated
// window, marks navigation ready for the reveal handshake.
func onMainWindowNavigation() {
	debugLog("navigation completed")
	markNavigationDone()
	showMainWindowAtStartup()
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
			setQuitting()
			flushBoundsSave()
			if getMainWindow() != nil {
				app.Event.Emit("app:quit", "")
				time.Sleep(250 * time.Millisecond)
			}
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
	visible := false
	if w := getMainWindow(); w != nil {
		visible = w.IsVisible()
	}
	debugLog("hotkey fired, visible=%v", visible)
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
//
// Returns true when a placement was applied, false when no screen geometry
// was available yet (the startup path then re-runs it shortly).
func positionWindowAtStartup() bool {
	if getMainWindow() == nil {
		return false
	}
	st := store.currentSettings()
	if st.WindowWidth > 0 && st.WindowHeight > 0 &&
		boundsVisible(st.WindowX, st.WindowY, st.WindowWidth, st.WindowHeight) {
		w := max(st.WindowWidth, minWidth)
		setWindowBounds(st.WindowX, st.WindowY, w, max(st.WindowHeight, minHeight))
		debugLog("position: restored saved bounds %d,%d %dx%d", st.WindowX, st.WindowY, w, st.WindowHeight)
		return true
	}
	// Primary may be nil if the screen cache has not been populated yet (it is
	// filled on ApplicationDidFinishLaunching). Never fall back to Center() here:
	// centering makes a sticky note feel like a dialog and differs from the
	// Windows "left edge, 1/3 width" default; prefer the first known screen and
	// keep the left-edge placement, or report "not placed" so the caller can
	// retry once screens are known.
	screen := app.Screen.GetPrimary()
	if screen == nil {
		if all := app.Screen.GetAll(); len(all) > 0 {
			screen = all[0]
		}
	}
	if screen == nil {
		debugLog("position: no screen info yet; deferring placement")
		return false
	}
	pwa := screen.PhysicalWorkArea
	width := pwa.Width / 3
	if width < minWidth {
		width = minWidth
	}
	setWindowBounds(pwa.X, pwa.Y, width, pwa.Height)
	debugLog("position: default left-edge 1/3 placement %d,%d %dx%d (primary=%v)",
		pwa.X, pwa.Y, width, pwa.Height, screen.IsPrimary)
	return true
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
// hotkey) do not yank the window back to the left edge. A silent start has no
// window, so the first summon claims it when it reveals that window.
var (
	positionedOnce bool
	positionMu     sync.Mutex
)

// markStartupPositioned records that the window has been placed for the first
// time (normal startup or the first silent summon).
func markStartupPositioned() {
	positionMu.Lock()
	positionedOnce = true
	positionMu.Unlock()
}

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
//
// In lightweight mode there may be no window at all: it is recreated and
// revealed once its page is ready, which is the small summon delay the
// setting warns about.
func showMainWindow() {
	if isQuitting() {
		return
	}
	windowStateMu.Lock()
	if destroying {
		revealAfterDestroy = true
		windowStateMu.Unlock()
		return
	}
	windowStateMu.Unlock()
	if isPendingReveal() {
		return // a recreation is already on its way
	}
	cancelLightweightTimer()
	w := getMainWindow()
	if w == nil {
		debugLog("show: recreating main window")
		setPendingReveal(true)
		if createMainWindowOnMainThread() == nil {
			setPendingReveal(false)
			return
		}
		time.AfterFunc(revealFallbackDelay, revealRecreatedWindowFallback)
		return
	}
	debugLog("show: emitting app:show")
	app.Event.Emit("app:show", "")
	w.Show()
	w.Focus()
}

// revealRecreatedWindowFallback shows the window even if the ready handshake
// never arrives (a broken page must not leave the app unreachable).
func revealRecreatedWindowFallback() {
	windowStateMu.Lock()
	if !pendingReveal {
		windowStateMu.Unlock()
		return
	}
	pendingReveal = false
	windowStateMu.Unlock()
	revealMainWindow()
}

// showMainWindowAtStartup positions the window once the WebView has settled
// and shows it (unless this is a silent launch). Runs at most once; if screen
// geometry was not available yet, the placement itself is retried shortly.
func showMainWindowAtStartup() {
	// A recreated window is handled by the reveal handshake; a silent start
	// has no window yet and stays without one until first summon.
	if isPendingReveal() || getMainWindow() == nil {
		return
	}
	positionMu.Lock()
	if positionedOnce {
		positionMu.Unlock()
		return
	}
	positionedOnce = true
	positionMu.Unlock()

	if !positionWindowAtStartup() {
		// Screen cache not populated yet (macOS fills it after launch): re-run
		// the placement once it is, so a sticky note never stays centered on
		// the very first boot. The window may already be visible by then; the
		// re-position is what the user expects from "remember my window".
		time.AfterFunc(500*time.Millisecond, func() {
			positionWindowAtStartup()
		})
	}
	applyWindowOpacity()
	if !silentStart {
		showMainWindow()
	} else {
		// Silent launch (auto-start): the window is born hidden and no hide
		// event fires, so start both the working-set trim and the lightweight
		// teardown countdown here.
		trimWorkingSetAfterHide()
		scheduleLightweightDestroy()
	}
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
	tray = app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Slite Note")
	// On macOS the label renders as text next to the status-bar icon (NSStatusItem
	// title), which eats menu-bar space and looks wrong for a sticky-note tray;
	// Windows ignores SetLabel entirely (empty impl), so skip it everywhere.
	if runtime.GOOS != "darwin" {
		tray.SetLabel("Slite Note")
	}

	// Labels follow the persisted language setting when it names a concrete
	// locale; "" (follow the OS) starts in English until the frontend resolves
	// the locale and pushes it back via SetTrayLanguage.
	applyTrayLanguage(store.currentSettings().Language)

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
	w := getMainWindow()
	if w == nil {
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
		for !w.IsVisible() {
			if time.Now().After(deadline) {
				log.Println("SMOKE FAIL: window not visible after show")
				os.Exit(1)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	debugLog("smoke ok")
	fmt.Println("SMOKE OK")
	setQuitting()
	app.Quit()
	os.Exit(0)
}

// toggleWindow shows or hides the main window (hotkey, tray click, tray menu).
// A destroyed window counts as hidden and is recreated by showMainWindow.
func toggleWindow() {
	if w := getMainWindow(); w != nil && w.IsVisible() {
		hideWindow()
		return
	}
	showMainWindow()
}

func hideWindow() {
	w := getMainWindow()
	if w == nil {
		cancelLightweightTimer()
		return
	}
	// Let the frontend flush pending auto-saves before the window disappears.
	app.Event.Emit("app:hide", "")
	flushBoundsSave()
	w.Hide()
	// Release the (cold) physical working set of this process and the WebView2
	// children; Windows-side only, no-op on macOS (see platform_windows.go).
	// The WindowHide event schedules the lightweight teardown for hides that
	// bypass this function (the frontend's title-bar button calls Wails
	// Window.Hide directly).
	trimWorkingSetAfterHide()
	scheduleLightweightDestroy()
}
