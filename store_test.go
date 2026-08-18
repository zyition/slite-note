package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return &Store{dataDir: t.TempDir(), settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
}

func TestSaveLoadNotesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	notes := []Note{{ID: "a", Blocks: []map[string]any{{"type": "paragraph"}}, CreatedAt: 1, UpdatedAt: 2}}
	if err := s.SaveNotes(notes); err != nil {
		t.Fatalf("SaveNotes: %v", err)
	}
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestLoadNotesMissingFile(t *testing.T) {
	s := newTestStore(t)
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes on missing file should not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty notes, got %d", len(got))
	}
}

func TestLoadNotesCorruptRecovery(t *testing.T) {
	s := newTestStore(t)
	corrupt := []byte("{not json!!!")
	if err := os.WriteFile(s.notesPath(), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("corrupt file should recover without error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty notes after recovery, got %d", len(got))
	}
	// The original bytes must be preserved in a backup file.
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	var backupFound bool
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "notes.json.corrupt-") {
			backupFound = true
			data, err := os.ReadFile(filepath.Join(s.dataDir, e.Name()))
			if err != nil || string(data) != string(corrupt) {
				t.Fatalf("backup content mismatch: %v", err)
			}
		}
	}
	if !backupFound {
		t.Fatal("expected a notes.json.corrupt-* backup file")
	}
}

func TestSaveSettingsOpacityClamp(t *testing.T) {
	s := newTestStore(t)
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 1}, {0.2, 1}, {0.3, 0.3}, {0.5, 0.5}, {1, 1}, {1.5, 1},
	}
	for _, c := range cases {
		st := Settings{Theme: "system", Opacity: c.in}
		if err := s.SaveSettings(st); err != nil {
			t.Fatalf("SaveSettings(opacity=%v): %v", c.in, err)
		}
		if got := s.settings.Opacity; got != c.want {
			t.Errorf("opacity %v → %v, want %v", c.in, got, c.want)
		}
	}
}
