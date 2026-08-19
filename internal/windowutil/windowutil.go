// Package windowutil holds pure, platform-free helpers extracted from the
// Win32 window code so the geometry / clamping / hotkey-string logic can be
// unit-tested without a Windows build (no golang.org/x/sys/windows imports).
package windowutil

import (
	"errors"
	"strings"
)

// OpacityFloor is the UI slider minimum (Settings.opacity below this or above
// 1 means "not set" → fully opaque).
const OpacityFloor = 0.3

// ClampOpacity normalizes a persisted opacity value to the valid range:
// anything below the slider floor or above 1 is treated as unset and returns
// fully opaque. setWindowOpacity has its own lower clamp (opacityMin); this
// function guards the settings layer only.
func ClampOpacity(op float64) float64 {
	if op < OpacityFloor || op > 1 {
		return 1
	}
	return op
}

// ScreenArea is the physical work area of one monitor (physical pixels).
type ScreenArea struct {
	X, Y, Width, Height int
}

// RectOverlapsAny reports whether the rect (x, y, w, h) overlaps any of the
// given screen work areas, so a window left on a disconnected monitor falls
// back to the default placement instead of reopening off-screen.
func RectOverlapsAny(x, y, w, h int, screens []ScreenArea) bool {
	for _, s := range screens {
		ox := max(x, s.X)
		oy := max(y, s.Y)
		ox2 := min(x+w, s.X+s.Width)
		oy2 := min(y+h, s.Y+s.Height)
		if ox2 > ox && oy2 > oy {
			return true
		}
	}
	return false
}

// NormalizeHotkey trims and validates a user-supplied global shortcut combo.
// Returns an error for an empty combo (the only invalid case the settings UI
// can produce; registration failures are caught later by GlobalShortcut).
func NormalizeHotkey(combo string) (string, error) {
	combo = strings.TrimSpace(combo)
	if combo == "" {
		return "", errors.New("hotkey must not be empty")
	}
	return combo, nil
}
