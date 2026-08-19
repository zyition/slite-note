package windowutil

import "testing"

// --- RectOverlapsAny (window-off-screen fallback geometry) ---

func TestRectOverlapsAnyFullyInside(t *testing.T) {
	screens := []ScreenArea{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	if !RectOverlapsAny(100, 100, 480, 480, screens) {
		t.Fatal("window fully inside the work area must overlap")
	}
}

func TestRectOverlapsAnyPartial(t *testing.T) {
	screens := []ScreenArea{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	// Window hanging off the right edge still overlaps.
	if !RectOverlapsAny(1800, 100, 480, 480, screens) {
		t.Fatal("window crossing the work-area edge must overlap")
	}
}

func TestRectOverlapsAnyOffScreen(t *testing.T) {
	screens := []ScreenArea{{X: 0, Y: 0, Width: 1920, Height: 1080}}
	// Fully to the right of the screen (classic disconnected-monitor case).
	if RectOverlapsAny(2000, 100, 480, 480, screens) {
		t.Fatal("window fully off-screen must not overlap")
	}
}

func TestRectOverlapsAnyNoScreens(t *testing.T) {
	if RectOverlapsAny(0, 0, 100, 100, nil) {
		t.Fatal("empty screen list must report no overlap")
	}
}

func TestRectOverlapsAnySecondaryMonitorNegativeX(t *testing.T) {
	// Primary at x=0, secondary to its LEFT (negative physical coords).
	screens := []ScreenArea{
		{X: 0, Y: 0, Width: 1920, Height: 1080},
		{X: -1280, Y: 0, Width: 1280, Height: 1080},
	}
	if !RectOverlapsAny(-1200, 500, 400, 400, screens) {
		t.Fatal("window on the negative-x secondary monitor must overlap")
	}
	if RectOverlapsAny(2000, 500, 400, 400, screens) {
		t.Fatal("window right of all screens must not overlap")
	}
}

func TestRectOverlapsAnyTouchingEdgeIsNotOverlap(t *testing.T) {
	screens := []ScreenArea{{X: 0, Y: 0, Width: 100, Height: 100}}
	// Window starts exactly at the right edge: zero overlap area.
	if RectOverlapsAny(100, 0, 50, 50, screens) {
		t.Fatal("edge-touching rect has zero overlap and must not count")
	}
	// One pixel inside counts.
	if !RectOverlapsAny(99, 0, 50, 50, screens) {
		t.Fatal("one-pixel overlap must count")
	}
}

// --- ClampOpacity (settings-layer opacity normalization) ---

func TestClampOpacityKeepsValidRange(t *testing.T) {
	for _, op := range []float64{0.3, 0.5, 0.75, 1} {
		if got := ClampOpacity(op); got != op {
			t.Errorf("ClampOpacity(%v) = %v, want %v", op, got, op)
		}
	}
}

func TestClampOpacityDefaultsToOpaque(t *testing.T) {
	for _, op := range []float64{0, 0.29, -0.5, 1.01, 2} {
		if got := ClampOpacity(op); got != 1 {
			t.Errorf("ClampOpacity(%v) = %v, want 1 (unset/out-of-range → opaque)", op, got)
		}
	}
}

// --- NormalizeHotkey (settings combo validation) ---

func TestNormalizeHotkeyTrimAndKeep(t *testing.T) {
	got, err := NormalizeHotkey("  Ctrl+Shift+K  ")
	if err != nil || got != "Ctrl+Shift+K" {
		t.Fatalf("NormalizeHotkey trimmed = %q, err=%v, want Ctrl+Shift+K", got, err)
	}
	got, err = NormalizeHotkey("Alt+Shift+S")
	if err != nil || got != "Alt+Shift+S" {
		t.Fatalf("NormalizeHotkey kept = %q, err=%v", got, err)
	}
}

func TestNormalizeHotkeyEmptyRejected(t *testing.T) {
	for _, combo := range []string{"", "   ", "\t"} {
		if _, err := NormalizeHotkey(combo); err == nil {
			t.Errorf("NormalizeHotkey(%q) should error", combo)
		}
	}
}
