//go:build windows

package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/zyition/slite-note/internal/windowutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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

// systemPrefersDark reports the OS app-mode preference (registry
// AppsUseLightTheme). Only feeds the window-creation background; the frontend
// re-syncs the real theme through its own matchMedia listener.
func systemPrefersDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}

// --- hide-time working-set trim (shrink physical RAM while hidden) ---
//
// A hidden slite-note is cold: nothing paints until summoned. Windows only
// pages cold pages out under memory pressure, which on roomy machines may
// never happen, so the whole WebView2 tree (browser + renderer) sits in
// physical RAM indefinitely. Trimming on hide releases it immediately; the
// pages come back via (memory-compressed) soft faults on the next show —
// imperceptible for a window this small. The trade is deliberate: this is a
// mostly-hidden always-on app. macOS counterpart is a no-op (see
// platform_darwin.go) — App Nap + macOS memory compression already do this.

// Win32 QUOTA_LIMITS_* flags (not exposed by golang.org/x/sys/windows): the
// DISABLE pair makes the trim soft — the working set can grow back freely
// afterwards instead of being capped.
const (
	quotaHardWsMinDisable = 0x00000002
	quotaHardWsMaxDisable = 0x00000008
)

// trimHideDelay is the debounce window for the hide-time working-set trim.
const trimHideDelay = 2 * time.Second

// trimTimer arms a trailing-edge debounce: every hideWindow() call pushes the
// trim trimHideDelay further out, so it runs once, trimHideDelay after the
// LAST hide. Guarded by a mutex because Reset/nil-ing must not race with the
// callback — schedule() callers (hotkey dispatch runs on its own goroutine,
// see Wails' GlobalShortcutManager.dispatch) and the AfterFunc callback are
// always different goroutines.
type trimDebounce struct {
	delay time.Duration

	mu    sync.Mutex
	timer *time.Timer

	// hooks, overridden in tests (platform_windows_test.go)
	windowHidden func() bool // precondition: trim only while hidden
	run          func()      // the actual trim
}

// schedule coalesces requests into one run, delay after the last request.
func (d *trimDebounce) schedule() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Reset(d.delay) // re-arm: fire d.delay after the LAST request
		return
	}
	d.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		d.timer = nil
		d.mu.Unlock()
		// A re-show within the delay means the user wants the window back:
		// skip the trim entirely instead of evicting pages of a visible
		// window (immediate fault-back = repaint stutter).
		if d.windowHidden() {
			d.run()
		}
	})
}

// trimWorkingSetAfterHide is the platform entry point called from
// hideWindow(). One-shot and debounced: trimming evicts hot pages along with
// cold ones, and background work faults the hot subset back in over the
// following seconds — the working set settles at its genuinely-active size
// (~20MB) and stays there. That plateau is the OS finding the real hot set;
// re-trimming on top of it would just evict live pages and churn CPU for a
// few MB of bookkeeping.
func trimWorkingSetAfterHide() {
	trimSchedule.schedule()
}

// trimSchedule is the production debouncer; trimHideDelay gives the user time
// to change their mind: a quick hotkey re-show within the window skips the
// trim entirely. Nothing here is latency-sensitive — the only cost of a
// longer delay is reclaiming the working set a little later, which is
// invisible at minutes-timescale hiding.
var trimSchedule = &trimDebounce{
	delay:        trimHideDelay,
	windowHidden: func() bool { return mainWindow == nil || !mainWindow.IsVisible() },
	run:          trimWorkingSet,
}

// trimWorkingSet empties the working set of this process and every descendant
// (WebView2 spawns msedgewebview2.exe children that hold the real memory).
func trimWorkingSet() {
	trimProcessWorkingSet(windows.CurrentProcess())
	pids := descendantProcessIDs(uint32(os.Getpid()))
	for _, pid := range pids {
		h, err := windows.OpenProcess(
			windows.PROCESS_SET_QUOTA|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			continue // exited meanwhile; best effort
		}
		trimProcessWorkingSet(h)
		windows.CloseHandle(h)
	}
	debugLog("trim: self + %d descendant(s)", len(pids))
}

// trimProcessWorkingSet requests an empty working set (SIZE_T max = "remove
// everything pageable"). Best effort; failures are harmless.
func trimProcessWorkingSet(h windows.Handle) {
	_ = windows.SetProcessWorkingSetSizeEx(h, ^uintptr(0), ^uintptr(0),
		quotaHardWsMinDisable|quotaHardWsMaxDisable)
}

// descendantProcessIDs walks the process snapshot and returns every PID
// descending from rootPID (WebView2 children may nest one level deep).
func descendantProcessIDs(rootPID uint32) []uint32 {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	children := make(map[uint32][]uint32)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID != 0 {
			children[entry.ParentProcessID] = append(children[entry.ParentProcessID], entry.ProcessID)
		}
	}

	var out []uint32
	queue := []uint32{rootPID}
	seen := map[uint32]bool{rootPID: true}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if !seen[child] {
				seen[child] = true
				out = append(out, child)
				queue = append(queue, child)
			}
		}
	}
	return out
}
