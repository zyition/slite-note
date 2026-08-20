package main

import (
	"encoding/json"
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
	note := Note{ID: "a", Blocks: []map[string]any{{"type": "paragraph"}}, CreatedAt: 1, UpdatedAt: 2}
	if err := s.SaveNote(note); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	// Per-note layout: the note lives in notes/<id>.json.
	if _, err := os.Stat(filepath.Join(s.dataDir, "notes", "a.json")); err != nil {
		t.Fatalf("expected notes/a.json: %v", err)
	}
}

func TestSaveNoteDeleteNote(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveNote(Note{ID: "x", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNote("x"); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}
	got, err := s.LoadNotes()
	if err != nil || len(got) != 0 {
		t.Fatalf("after delete: got=%d err=%v, want empty", len(got), err)
	}
	// Deleting a missing note is idempotent, not an error.
	if err := s.DeleteNote("x"); err != nil {
		t.Fatalf("second DeleteNote should be idempotent: %v", err)
	}
	// Empty id is rejected.
	if err := s.SaveNote(Note{ID: "", CreatedAt: 1}); err == nil {
		t.Fatal("SaveNote with empty id must fail")
	}
}

// TestMigrateNotesJsonToPerNote: the legacy single-file layout is split into
// notes/<id>.json on first LoadNotes, and the original file is renamed to a
// backup — never deleted.
func TestMigrateNotesJsonToPerNote(t *testing.T) {
	s := newTestStore(t)
	legacy := `{"version":1,"notes":[{"id":"b","title":"two","blocks":[],"createdAt":2,"updatedAt":2},{"id":"a","title":"one","blocks":[],"createdAt":1,"updatedAt":1}]}`
	if err := os.WriteFile(s.notesPath(), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes after migration: %v", err)
	}
	// Sorted by createdAt regardless of legacy array order.
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("migrated+sorted notes mismatch: %+v", got)
	}
	// Per-note files exist; legacy file backed up (not deleted).
	if _, err := os.Stat(filepath.Join(s.dataDir, "notes", "a.json")); err != nil {
		t.Errorf("notes/a.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.dataDir, "notes", "b.json")); err != nil {
		t.Errorf("notes/b.json missing: %v", err)
	}
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	var backup string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "notes.json.migrated-") {
			backup = e.Name()
		}
	}
	if backup == "" {
		t.Fatal("expected a notes.json.migrated-* backup")
	}
	// Second load reads per-note files and does not re-migrate.
	got2, err := s.LoadNotes()
	if err != nil || len(got2) != 2 {
		t.Fatalf("second load: got=%d err=%v", len(got2), err)
	}
}

// TestMigrateNotesJsonFillsMissingIDs: legacy data without an id gets one
// during migration (defensive; the frontend normally generates UUIDs).
func TestMigrateNotesJsonFillsMissingIDs(t *testing.T) {
	s := newTestStore(t)
	legacy := `{"version":1,"notes":[{"title":"no id","blocks":[],"createdAt":1,"updatedAt":1}]}`
	if err := os.WriteFile(s.notesPath(), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("LoadNotes: %v", err)
	}
	if len(got) != 1 || got[0].ID == "" {
		t.Fatalf("missing id not filled: %+v", got)
	}
}

// TestLoadNotesSortsByCreatedAt: per-note files are read in arbitrary
// directory order, so the result must be sorted explicitly.
func TestLoadNotesSortsByCreatedAt(t *testing.T) {
	s := newTestStore(t)
	for _, n := range []Note{
		{ID: "mid", CreatedAt: 5, UpdatedAt: 5},
		{ID: "old", CreatedAt: 1, UpdatedAt: 1},
		{ID: "new", CreatedAt: 9, UpdatedAt: 9},
	} {
		if err := s.SaveNote(n); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	if ids[0] != "old" || ids[1] != "mid" || ids[2] != "new" {
		t.Fatalf("sort order mismatch: %v", ids)
	}
}

// TestLoadNotesSkipsCorruptNote: one corrupt per-note file must not break
// the rest (per-note isolation — the point of the layout).
func TestLoadNotesSkipsCorruptNote(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveNote(Note{ID: "ok", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.dataDir, "notes", "bad.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadNotes()
	if err != nil {
		t.Fatalf("corrupt note should be skipped, not fatal: %v", err)
	}
	if len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("expected only the healthy note: %+v", got)
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

// TestMoveDataDirSurvivesRestart guards the bootstrap contract: after a move
// the data (notes.json + settings.json) lives in the target dir, and the
// default dir's app.json — the only file NewStore reads at startup — points
// at it, so a restart lands on the custom dir.
func TestMoveDataDirSurvivesRestart(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(defaultDir, "settings.json"), []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.MoveDataDir(customDir); err != nil {
		t.Fatalf("MoveDataDir: %v", err)
	}
	if s.dataDir != customDir {
		t.Fatalf("active dir after move = %q, want %q", s.dataDir, customDir)
	}

	// Restart: the default dir's app.json is the single source of truth.
	restarted := &Store{dataDir: defaultDir, defaultDir: defaultDir}
	if d := restarted.readAppConfigDataDir(); !strings.EqualFold(filepath.Clean(d), filepath.Clean(customDir)) {
		t.Fatalf("app.json DataDir = %q, want %q", d, customDir)
	}
	// Move semantics: data left the default dir, the custom dir holds it.
	for _, f := range []string{"notes.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(defaultDir, f)); !os.IsNotExist(err) {
			t.Errorf("default dir %s should have been moved away, err=%v", f, err)
		}
		if _, err := os.Stat(filepath.Join(customDir, f)); err != nil {
			t.Errorf("%s not migrated to custom dir: %v", f, err)
		}
	}
}

// TestMoveDataDirFollowsRepeatedMigration: default → A → B must leave the
// default dir's app.json pointing at B, so a restart lands on the latest
// custom dir (not the first one).
func TestMoveDataDirFollowsRepeatedMigration(t *testing.T) {
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
	if err := s.MoveDataDir(dirA); err != nil {
		t.Fatalf("move to A: %v", err)
	}
	if err := s.MoveDataDir(dirB); err != nil {
		t.Fatalf("move to B: %v", err)
	}

	restarted := &Store{dataDir: defaultDir, defaultDir: defaultDir}
	if d := restarted.readAppConfigDataDir(); !strings.EqualFold(filepath.Clean(d), filepath.Clean(dirB)) {
		t.Fatalf("app.json DataDir = %q, want %q (must follow repeated moves)", d, dirB)
	}
}

// TestMoveDataDirIntoDefault: migrating back into the default dir is allowed;
// app.json then points at the default dir itself (no-op for discovery).
func TestMoveDataDirIntoDefault(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default")
	customDir := filepath.Join(base, "custom")
	for _, d := range []string{defaultDir, customDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.MoveDataDir(customDir); err != nil {
		t.Fatalf("move to custom: %v", err)
	}
	if err := s.MoveDataDir(defaultDir); err != nil {
		t.Fatalf("move back to default: %v", err)
	}
	if s.dataDir != defaultDir {
		t.Fatalf("active dir = %q, want %q", s.dataDir, defaultDir)
	}
	restarted := &Store{dataDir: defaultDir, defaultDir: defaultDir}
	// Migrating back to the default dir stores the canonical "" pointer.
	if d := restarted.readAppConfigDataDir(); d != "" {
		t.Fatalf("app.json DataDir = %q, want %q (default form)", d, "")
	}
}

// TestUseDataDirAdoptsExisting: UseDataDir points app.json at an existing
// slite directory and reloads its preferences, without copying or deleting
// anything — the reinstall/new-machine reconnect path.
func TestUseDataDirAdoptsExisting(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default")
	customDir := filepath.Join(base, "custom")
	for _, d := range []string{defaultDir, customDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The existing dir has its own notes and dark-theme preferences.
	if err := os.WriteFile(filepath.Join(customDir, "notes.json"), []byte(`{"version":1,"notes":[{"id":"x","blocks":[],"createdAt":5,"updatedAt":5}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "settings.json"), []byte(`{"theme":"dark","hotkey":"Ctrl+Shift+X"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.UseDataDir(customDir); err != nil {
		t.Fatalf("UseDataDir: %v", err)
	}
	if s.dataDir != customDir {
		t.Fatalf("active dir = %q, want %q", s.dataDir, customDir)
	}
	// Preferences must reload from the adopted dir, not stay at the current ones.
	if s.settings.Theme != "dark" || s.settings.Hotkey != "Ctrl+Shift+X" {
		t.Fatalf("preferences not reloaded from adopted dir: %+v", s.settings)
	}
	// Nothing copied, nothing deleted.
	if _, err := os.Stat(filepath.Join(customDir, "notes.json")); err != nil {
		t.Errorf("adopted notes.json should remain untouched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(customDir, "settings.json")); err != nil {
		t.Errorf("adopted settings.json should remain untouched: %v", err)
	}
}

// TestUseDataDirIntoEmpty: adopting an empty folder means a fresh data dir —
// allowed, points app.json there, preferences fall back to defaults.
func TestUseDataDirIntoEmpty(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default")
	emptyDir := filepath.Join(base, "empty")
	for _, d := range []string{defaultDir, emptyDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := &Store{dataDir: defaultDir, defaultDir: defaultDir, settings: Settings{Theme: "system", Hotkey: defaultHotkey}}
	if err := s.UseDataDir(emptyDir); err != nil {
		t.Fatalf("UseDataDir into empty: %v", err)
	}
	restarted := &Store{dataDir: defaultDir, defaultDir: defaultDir}
	if d := restarted.readAppConfigDataDir(); !strings.EqualFold(filepath.Clean(d), filepath.Clean(emptyDir)) {
		t.Fatalf("app.json DataDir = %q, want %q", d, emptyDir)
	}
}

// TestValidateDataDirModes: move mode requires an empty target (never
// overwrite); adopt mode allows an empty folder or one holding only
// slite-owned files, and rejects foreign content.
func TestValidateDataDirModes(t *testing.T) {
	base := t.TempDir()
	s := &Store{dataDir: filepath.Join(base, "active"), defaultDir: filepath.Join(base, "active")}

	emptyDir := filepath.Join(base, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.validateDataDir(emptyDir, true); err != nil {
		t.Errorf("move into empty dir should pass: %v", err)
	}
	if err := s.validateDataDir(emptyDir, false); err != nil {
		t.Errorf("adopt empty dir should pass: %v", err)
	}

	// slite-owned content: rejected for move, accepted for adopt.
	sliteDir := filepath.Join(base, "slite-content")
	if err := os.MkdirAll(sliteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"notes.json", "settings.json", "notes.json.tmp", "log.txt"} {
		if err := os.WriteFile(filepath.Join(sliteDir, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.validateDataDir(sliteDir, true); err == nil {
		t.Error("move into non-empty dir must be rejected")
	}
	if err := s.validateDataDir(sliteDir, false); err != nil {
		t.Errorf("adopt dir with slite-owned files should pass: %v", err)
	}

	// Foreign content: rejected in both modes.
	foreignDir := filepath.Join(base, "foreign")
	if err := os.MkdirAll(foreignDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreignDir, "photo.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.validateDataDir(foreignDir, false); err == nil {
		t.Error("adopt dir with foreign files must be rejected")
	}
}

// TestLegacyAnchorMigratesToAppConfig: a pre-app.json install kept its custom
// DataDir inside the default dir's settings.json. First boot after upgrade
// must migrate that into app.json so the boot path is uniform.
func TestLegacyAnchorMigratesToAppConfig(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "default")
	customDir := filepath.Join(base, "custom")
	for _, d := range []string{defaultDir, customDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	legacy := []byte(`{"theme":"dark","dataDir":"` + filepath.ToSlash(customDir) + `"}`)
	if err := os.WriteFile(filepath.Join(defaultDir, "settings.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Store{dataDir: defaultDir, defaultDir: defaultDir}
	if d := s.readAppConfigDataDir(); !strings.EqualFold(filepath.Clean(d), filepath.Clean(customDir)) {
		t.Fatalf("migrated DataDir = %q, want %q", d, customDir)
	}
	// app.json now holds the pointer.
	cfg, err := os.ReadFile(filepath.Join(defaultDir, "app.json"))
	if err != nil {
		t.Fatalf("app.json not written: %v", err)
	}
	var parsed AppConfig
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatalf("app.json unparseable: %v", err)
	}
	if !strings.EqualFold(filepath.Clean(parsed.DataDir), filepath.Clean(customDir)) {
		t.Fatalf("app.json DataDir = %q, want %q", parsed.DataDir, customDir)
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
	s.pickExportDir = func() (string, error) { return dir, nil }

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
	s.pickExportDir = func() (string, error) { return "", nil }
	got, err := s.ExportAllMarkdown([]MarkdownFile{{Name: "x", Content: "y"}})
	if err != nil || got != 0 {
		t.Fatalf("cancelled export: got=%d err=%v, want 0 nil", got, err)
	}
}

// TestExportAllMarkdownSingleNote: a one-element slice is the single-note
// export path (same picker + naming as bulk).
func TestExportAllMarkdownSingleNote(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	s.pickExportDir = func() (string, error) { return dir, nil }
	got, err := s.ExportAllMarkdown([]MarkdownFile{{Name: "My note", Content: "# hi"}})
	if err != nil || got != 1 {
		t.Fatalf("single export: got=%d err=%v, want 1 nil", got, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "My note.md"))
	if err != nil || string(data) != "# hi" {
		t.Errorf("single export content: %q, err=%v", data, err)
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

// TestAppVersionDefault guards the ldflags -X injection contract: appVersion
// must never be empty, and a non-release build reports "dev" (a release build
// overrides it via -ldflags "-X main.appVersion=vX.Y.Z"; CI passes the tag).
func TestAppVersionDefault(t *testing.T) {
	if appVersion == "" {
		t.Fatal("appVersion must not be empty")
	}
	// Without -ldflags injection the default must be "dev", not a stale
	// hard-coded release number (that was the regression: store.go shipped
	// "0.2.0" while releases had moved on).
	if appVersion != "dev" {
		t.Fatalf("default appVersion = %q, want %q (release builds inject via ldflags)", appVersion, "dev")
	}
}
