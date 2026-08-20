//go:build windows

package main

import (
	"os"
	"path/filepath"
	"unsafe"

	"github.com/zyition/slite-note/internal/windowutil"
	"golang.org/x/sys/windows"
)

// --- window opacity (Win32 WS_EX_LAYERED + SetLayeredWindowAttributes) ---
//
// Wails v3 has no opacity API, so we drive the window through user32 directly.
// alpha must be in (0, 1]; values below 0.05 remove the layered style (fully
// opaque). The whole window — WebView2 content included — is affected.
// (macOS has its own opacity path in platform_darwin.go.)

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
	if err := setWindowOpacity(windowutil.ClampOpacity(store.currentSettings().Opacity)); err != nil {
		debugLog("set opacity failed: %v", err)
	}
}

// setWindowBounds moves/resizes the native window via SetWindowPos (raw Win32:
// the Wails setBounds path re-applies LWA_ALPHA=255 and is unreliable for a
// still-hidden window).
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

// setupPlatformUI wires platform-specific chrome. Windows: no application
// menu (the frameless window has no menu bar; the tray owns app actions).
func setupPlatformUI() {}

// registerPlatformHooks wires platform lifecycle events. Windows: the tray's
// Quit already flushes pending bounds; nothing else to hook.
func registerPlatformHooks() {}
