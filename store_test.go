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

// TestSetDataDirSurvivesRestart guards the DataDir anchor regression:
// SetDataDir used to delete the default dir's settings.json — the only file
// NewStore reads at startup to rediscover a custom data dir — so any restart
// fell back to the default directory.
func TestSetDataDirSurvivesRestart(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default") // stands in for %APPDATA%\slite
	customDir := filepath.Join(base, "custom")
	for _, d := range []string{defaultDir, customDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	notes := []byte(`{"version":1,"notes":[{"id":"a","title":"hi","blocks":[],"createdAt":1,"updatedAt":1}]}`)
	if err := os.WriteFile(filepath.Join(defaultDir, "notes.json"), notes, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.SetDataDir(customDir); err != nil {
		t.Fatalf("SetDataDir: %v", err)
	}
	if s.dataDir != customDir {
		t.Fatalf("active dir after migration = %q, want %q", s.dataDir, customDir)
	}

	// Restart: NewStore reads only the default dir's settings.json to learn
	// the custom DataDir. It must still point at customDir.
	st := (&Store{dataDir: defaultDir, defaultDir: defaultDir}).readSettingsFile(filepath.Join(defaultDir, "settings.json"))
	if st.DataDir == "" {
		t.Fatal("anchor lost: default dir settings.json has no DataDir after migration")
	}
	if !strings.EqualFold(filepath.Clean(st.DataDir), filepath.Clean(customDir)) {
		t.Fatalf("anchor DataDir = %q, want %q", st.DataDir, customDir)
	}

	// Move semantics: notes left the default dir, the anchor settings.json
	// stays; the custom dir holds the migrated notes plus its own settings.
	if _, err := os.Stat(filepath.Join(defaultDir, "notes.json")); !os.IsNotExist(err) {
		t.Errorf("default dir notes.json should have been moved away, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(defaultDir, "settings.json")); err != nil {
		t.Errorf("anchor settings.json should be kept in the default dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(customDir, "notes.json")); err != nil {
		t.Errorf("notes.json not migrated to custom dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(customDir, "settings.json")); err != nil {
		t.Errorf("settings.json not written to custom dir: %v", err)
	}
}

// TestSetDataDirAnchorFollowsRepeatedMigration: default → A → B must leave
// the default dir anchor pointing at B, so a restart lands on the latest
// custom dir (not the first one).
func TestSetDataDirAnchorFollowsRepeatedMigration(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default")
	dirA := filepath.Join(base, "a")
	dirB := filepath.Join(base, "b")
	for _, d := range []string{defaultDir, dirA, dirB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.SetDataDir(dirA); err != nil {
		t.Fatalf("migrate to A: %v", err)
	}
	if err := s.SetDataDir(dirB); err != nil {
		t.Fatalf("migrate to B: %v", err)
	}

	st := (&Store{dataDir: defaultDir, defaultDir: defaultDir}).readSettingsFile(filepath.Join(defaultDir, "settings.json"))
	if !strings.EqualFold(filepath.Clean(st.DataDir), filepath.Clean(dirB)) {
		t.Fatalf("anchor DataDir = %q, want %q (must follow repeated migrations)", st.DataDir, dirB)
	}
	// The intermediate dir is fully moved away (notes + settings).
	for _, f := range []string{"notes.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(dirA, f)); !os.IsNotExist(err) {
			t.Errorf("intermediate dir %s should have been moved away, err=%v", f, err)
		}
	}
}

// TestSetDataDirIntoDefaultClearsAnchor: migrating back into the default dir
// is allowed; the anchor then points at the default dir itself (no-op for
// NewStore's discovery, and harmless).
func TestSetDataDirIntoDefaultClearsAnchor(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default")
	customDir := filepath.Join(base, "custom")
	for _, d := range []string{defaultDir, customDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.SetDataDir(customDir); err != nil {
		t.Fatalf("migrate to custom: %v", err)
	}
	if err := s.SetDataDir(defaultDir); err != nil {
		t.Fatalf("migrate back to default: %v", err)
	}
	if s.dataDir != defaultDir {
		t.Fatalf("active dir = %q, want %q", s.dataDir, defaultDir)
	}
	st := (&Store{dataDir: defaultDir, defaultDir: defaultDir}).readSettingsFile(filepath.Join(defaultDir, "settings.json"))
	if !strings.EqualFold(filepath.Clean(st.DataDir), filepath.Clean(defaultDir)) {
		t.Fatalf("anchor DataDir = %q, want %q", st.DataDir, defaultDir)
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

// TestSaveWindowBoundsPersistsAndIsOwnedByGoSide: SaveWindowBounds must
// persist the four bounds fields, and a later SaveSettings from the frontend
// (loaded before the bounds changed) must not clobber them.
func TestSaveWindowBoundsPersistsAndIsOwnedByGoSide(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveWindowBounds(120, 340, 640, 900); err != nil {
		t.Fatalf("SaveWindowBounds: %v", err)
	}
	st, err := s.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if st.WindowX != 120 || st.WindowY != 340 || st.WindowWidth != 640 || st.WindowHeight != 900 {
		t.Fatalf("bounds mismatch: %+v", st)
	}

	// A stale frontend snapshot (bounds = 0) must not erase the Go-side value.
	if err := s.SaveSettings(Settings{Theme: "dark", Hotkey: defaultHotkey}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	st, _ = s.LoadSettings()
	if st.WindowX != 120 || st.WindowHeight != 900 {
		t.Fatalf("SaveSettings clobbered window bounds: %+v", st)
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Shopping list", "Shopping list"},
		{"a:b*c?<d>", "a_b_c__d_"},
		{"trailing. ", "trailing"},
		{"  spaced  ", "spaced"},
		{"", "Untitled"},
		{"   ", "Untitled"},
		{"....", "Untitled"},
		{"中文便签", "中文便签"},
	}
	for _, c := range cases {
		if got := sanitizeFileName(c.in); got != c.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestExportAllMarkdownWritesFilesWithDedupedNames: ExportAllMarkdown writes
// one .md per entry, sanitizes names, and appends a numeric suffix on
// collisions (case-insensitive, like NTFS).
func TestExportAllMarkdownWritesFilesWithDedupedNames(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	s.pickDir = func() (string, error) { return dir, nil }

	files := []MarkdownFile{
		{Name: "Todo", Content: "- a\n- b\n"},
		{Name: "todo", Content: "dup\n"},
		{Name: "bad:name?", Content: "sanitized\n"},
	}
	got, err := s.ExportAllMarkdown(files)
	if err != nil {
		t.Fatalf("ExportAllMarkdown: %v", err)
	}
	if got != 3 {
		t.Fatalf("exported %d files, want 3", got)
	}
	for _, f := range []string{"Todo.md", "Todo (2).md", "bad_name_.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "Todo.md"))
	if err != nil || string(data) != "- a\n- b\n" {
		t.Errorf("Todo.md content mismatch: %q, err=%v", data, err)
	}
}

// TestExportAllMarkdownCancel: a cancelled folder picker writes nothing.
func TestExportAllMarkdownCancel(t *testing.T) {
	s := newTestStore(t)
	s.pickDir = func() (string, error) { return "", nil }
	got, err := s.ExportAllMarkdown([]MarkdownFile{{Name: "x", Content: "y"}})
	if err != nil || got != 0 {
		t.Fatalf("cancelled export: got=%d err=%v, want 0 nil", got, err)
	}
}

func TestSaveMarkdownDialogWritesChosenPath(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	wanted := filepath.Join(dir, "out.md")
	s.pickSavePath = func(name string) (string, error) { return wanted, nil }
	path, err := s.SaveMarkdownDialog("default.md", "# hi\n")
	if err != nil || path != wanted {
		t.Fatalf("SaveMarkdownDialog: path=%q err=%v", path, err)
	}
	data, err := os.ReadFile(wanted)
	if err != nil || string(data) != "# hi\n" {
		t.Errorf("file content mismatch: %q, err=%v", data, err)
	}
}

func TestSaveMarkdownDialogCancel(t *testing.T) {
	s := newTestStore(t)
	s.pickSavePath = func(name string) (string, error) { return "", nil }
	path, err := s.SaveMarkdownDialog("x.md", "body")
	if err != nil || path != "" {
		t.Fatalf("cancelled save: path=%q err=%v, want empty nil", path, err)
	}
}

func TestOpenMarkdownDialogReadsFile(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "note.md")
	if err := os.WriteFile(src, []byte("# imported\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.pickOpenPath = func() (string, error) { return src, nil }
	content, err := s.OpenMarkdownDialog()
	if err != nil || content != "# imported\n" {
		t.Fatalf("OpenMarkdownDialog: content=%q err=%v", content, err)
	}
}

func TestOpenMarkdownDialogCancel(t *testing.T) {
	s := newTestStore(t)
	s.pickOpenPath = func() (string, error) { return "", nil }
	content, err := s.OpenMarkdownDialog()
	if err != nil || content != "" {
		t.Fatalf("cancelled open: content=%q err=%v, want empty nil", content, err)
	}
}

// TestSaveWindowBoundsReloadsFromDisk: the persisted bounds survive a
// restart (NewStore → LoadSettings), i.e. the write path is the real
// settings.json, not just memory.
func TestSaveWindowBoundsReloadsFromDisk(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveWindowBounds(10, 20, 800, 600); err != nil {
		t.Fatal(err)
	}
	loaded := (&Store{dataDir: s.dataDir}).readSettingsFile(s.settingsPath())
	if loaded.WindowX != 10 || loaded.WindowY != 20 || loaded.WindowWidth != 800 || loaded.WindowHeight != 600 {
		t.Fatalf("bounds not persisted to disk: %+v", loaded)
	}
}
