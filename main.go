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
)

// Wails embeds the built frontend into the binary (frontend/dist is produced by
// `pnpm build` / `wails3 dev`).
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed icons/tray.png
var trayIcon []byte

// App-wide events forwarded to the frontend so it can flush pending saves.
func init() {
	application.RegisterEvent[string]("app:hide")
	application.RegisterEvent[string]("app:quit")
}

const (
	hotkeyToggle = "Alt+Shift+S"
	defaultWidth = 360
)

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

func main() {
	store = NewStore()

	app = application.New(application.Options{
		Name:        "slite",
		Description: "A minimal desktop sticky notes app",
		Services: []application.Service{
			application.NewService(store),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	settings := store.currentSettings()
	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "slite",
		Width:            defaultWidth,
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

	// Global hotkey toggles window visibility system-wide (core feature).
	if err := app.GlobalShortcut.Register(hotkeyToggle, func() {
		debugLog("hotkey fired, visible=%v", mainWindow.IsVisible())
		toggleWindow()
	}); err != nil {
		app.Logger.Error("could not register global shortcut", "error", err)
		debugLog("hotkey register failed: %v", err)
	} else {
		debugLog("hotkey registered: %s", hotkeyToggle)
	}

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		positionWindowAtStartup()
		setupTray()
		mainWindow.Show()
		mainWindow.Focus()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// positionWindowAtStartup places the window at the left edge of the primary
// screen's work area, full height, with the MVP default width.
func positionWindowAtStartup() {
	screen := app.Screen.GetPrimary()
	if screen == nil {
		mainWindow.Center()
		return
	}
	wa := screen.WorkArea
	mainWindow.SetPosition(wa.X, wa.Y)
	mainWindow.SetSize(defaultWidth, wa.Height)
}

// setupTray creates the system tray icon with a menu (Show/Hide, Quit). Left
// click toggles window visibility; the menu mirrors the same actions.
func setupTray() {
	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("slite")
	tray.SetLabel("slite")

	menu := app.NewMenu()
	menu.Add("Show/Hide").OnClick(func(ctx *application.Context) {
		toggleWindow()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(ctx *application.Context) {
		// Give the frontend a moment to flush pending auto-saves.
		app.Event.Emit("app:quit", "")
		time.Sleep(250 * time.Millisecond)
		app.Quit()
	})
	tray.SetMenu(menu)
	tray.OnClick(toggleWindow)
	tray.Run()
}

// toggleWindow shows or hides the main window (hotkey, tray click, tray menu).
func toggleWindow() {
	if mainWindow.IsVisible() {
		hideWindow()
	} else {
		mainWindow.Show()
		mainWindow.Focus()
	}
}

func hideWindow() {
	// Let the frontend flush pending auto-saves before the window disappears.
	app.Event.Emit("app:hide", "")
	mainWindow.Hide()
}
