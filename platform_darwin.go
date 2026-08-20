//go:build darwin

package main

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// --- window opacity (macOS: background alpha, not whole-window) ---
//
// Windows fades the whole window (WS_EX_LAYERED); macOS keeps text crisp and
// makes only the note background see-through. The visual work happens in the
// frontend (CSS background alpha via the --bg-opacity variable) plus
// SetBackgroundColour(alpha) applied by bridge.setWindowBackground, so there
// is nothing to do natively. The window itself is created with
// MacBackdropTransparent (see main.go), making the shell transparent so the
// semi-transparent CSS background lets the desktop show through.

func setWindowOpacity(alpha float64) error { return nil }

// setOpacityOverride is a no-op on macOS: the frontend restores --bg-opacity
// to 100% while an app-modal overlay is open.
func setOpacityOverride(on bool) { opacityOverride = on }

func applyWindowOpacity() {}

// screenScaleFactor returns the primary screen's backing scale factor
// (2.0 on Retina, 1.0 otherwise). Wails reports screen work areas in physical
// pixels but the window Position/Size APIs in logical points, so bounds
// round-trips must convert.
func screenScaleFactor() float64 {
	s := app.Screen.GetPrimary()
	if s == nil || s.ScaleFactor <= 0 {
		return 1
	}
	return float64(s.ScaleFactor)
}

// setWindowBounds moves/resizes the native window (physical pixels → logical
// points). Wails' SetPosition/SetSize go through setBounds on the main thread
// (NSWindow setFrame), which is reliable on macOS even for a hidden window.
func setWindowBounds(x, y, w, h int) {
	if mainWindow == nil {
		return
	}
	sf := screenScaleFactor()
	mainWindow.SetPosition(int(float64(x)/sf), int(float64(y)/sf))
	mainWindow.SetSize(int(float64(w)/sf), int(float64(h)/sf))
}

// saveWindowBoundsNow reads the window bounds (logical points → physical
// pixels, matching the persisted units) and saves them. Best effort.
func saveWindowBoundsNow() {
	boundsSaveTimer = nil
	if mainWindow == nil {
		return
	}
	x, y := mainWindow.Position()
	w, h := mainWindow.Size()
	if w <= 0 || h <= 0 {
		return
	}
	sf := screenScaleFactor()
	if err := store.SaveWindowBounds(
		int(float64(x)*sf), int(float64(y)*sf),
		int(float64(w)*sf), int(float64(h)*sf),
	); err != nil {
		debugLog("save window bounds failed: %v", err)
	}
}

// userDownloadsDir returns the user's Downloads folder (macOS keeps it at
// ~/Downloads), or "" if the home directory cannot be resolved.
func userDownloadsDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Downloads")
	}
	return ""
}

// setupPlatformUI installs the macOS application menu bar. A mac app needs a
// menu: without one, Cmd+Q / Cmd+H / Cmd+W and the edit shortcuts don't work
// and the About box is unreachable. The AppMenu/FileMenu/EditMenu/WindowMenu
// roles provide the standard items (About, Hide, Quit, Close Window, undo/
// copy/paste, minimize/zoom); Settings… is added with the mac convention
// Cmd+, and Cmd+W (Close Window) hides the window rather than quitting —
// matching the Windows "close = hide" model (ADR-0003).
func setupPlatformUI() {
	menu := app.NewMenu()

	appMenu := menu.AddRole(application.AppMenu)
	appMenu.Add("Settings…").SetAccelerator("Cmd+,").OnClick(func(ctx *application.Context) {
		app.Event.Emit("app:open-settings", "")
		if !mainWindow.IsVisible() {
			mainWindow.Show()
		}
		mainWindow.Focus()
	})
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.Quit)

	menu.AddRole(application.FileMenu)   // Close Window (Cmd+W) → WindowClosing → hideWindow
	menu.AddRole(application.EditMenu)   // undo/cut/copy/paste/select-all for the editor
	menu.AddRole(application.WindowMenu) // minimize / zoom

	app.Menu.SetApplicationMenu(menu)
}

// registerPlatformHooks wires macOS lifecycle events. ApplicationWillTerminate
// fires on Cmd+Q (the Quit role) — flush the debounced bounds save so the last
// drag position survives, mirroring the Windows tray-quit flush. Clicking the
// Dock icon of a hidden window should summon it (standard mac behaviour).
func registerPlatformHooks() {
	app.Event.OnApplicationEvent(events.Mac.ApplicationWillTerminate, func(*application.ApplicationEvent) {
		flushBoundsSave()
	})
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) {
		showMainWindow()
	})
}
