package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zyition/slite-note/internal/windowutil"
)

// Note is a single editable sticky note. Blocks holds BlockNote block JSON.
type Note struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"` // manual title override; empty means derive from first line
	Blocks    []map[string]any `json:"blocks"`
	CreatedAt int64            `json:"createdAt"`
	UpdatedAt int64            `json:"updatedAt"`
}

// Settings holds app-level preferences persisted across restarts.
type Settings struct {
	Theme           string  `json:"theme"` // "system" | "yellow" | "gray" | "dark"
	AlwaysOnTop     bool    `json:"alwaysOnTop"`
	Hotkey          string  `json:"hotkey"`          // global toggle accelerator, e.g. "Alt+Shift+S"
	LaunchAtStartup bool    `json:"launchAtStartup"` // Windows Run key (HKCU)
	Opacity         float64 `json:"opacity"`         // window opacity 0.3–1.0, 1 = opaque

	// Window bounds in physical pixels, persisted (debounced) on move/resize
	// so the window reopens where the user left it. 0 = never saved yet.
	// Owned by the Go side (SaveWindowBounds); the frontend must not set them.
	WindowX      int `json:"windowX,omitempty"`
	WindowY      int `json:"windowY,omitempty"`
	WindowWidth  int `json:"windowWidth,omitempty"`
	WindowHeight int `json:"windowHeight,omitempty"`
}

// appConfigVersion is the schema version of app.json (the bootstrap pointer).
const appConfigVersion = 1

// AppConfig is the bootstrap pointer persisted at %APPDATA%\slite\app.json —
// the only state that must never move with the data: it tells NewStore where
// the data lives. Preferences (settings.json) live inside the data dir and
// follow it on migration; this pointer does not, by design.
//
// DataDir "" means the default location (os.UserConfigDir()/slite). A custom
// directory survives uninstall/reinstall because the NSIS uninstaller keeps
// %APPDATA%\slite intact.
type AppConfig struct {
	Version int    `json:"version"`
	DataDir string `json:"dataDir"` // "" = default
}

// appVersion is the user-facing version shown in the About section. It is
// injected at build time via -ldflags "-X main.appVersion=vX.Y.Z" (the release
// CI passes the git tag; see build/windows/Taskfile.yml), so a var (not const)
// is required. "dev" marks a local / non-release build and is also what
// `go run` / `go test` report.
var appVersion = "dev"

const notesFileVersion = 1

type notesFile struct {
	Version int    `json:"version"`
	Notes   []Note `json:"notes"`
}

// Store is the persistence service bound to the frontend. It owns the data
// directory (os.UserConfigDir()/slite by default, configurable) and
// reads/writes notes.json and settings.json. The frontend calls these methods
// via generated bindings; in pure-browser fallback mode the frontend uses
// localStorage instead.
type Store struct {
	mu      sync.Mutex
	dataDir string
	// defaultDir is os.UserConfigDir()/slite — the bootstrap directory whose
	// app.json tells NewStore where the data lives (the data dir itself when
	// DataDir is ""). Its app.json is the single source of truth for
	// rediscovering a custom data directory after restart or reinstall.
	defaultDir string
	settings   Settings

	// hotkeyReconfigure is injected by main.go; it re-registers the global
	// toggle hotkey without disturbing the existing binding on failure.
	hotkeyReconfigure func(combo string) error
	// hotkeySuspend/hotkeyResume temporarily unregister/restore the global
	// toggle hotkey while the user records a new combo in the settings panel.
	hotkeySuspend func() error
	hotkeyResume  func() error
	// pickDir opens the native folder picker (Windows) for data migration.
	pickDir func() (string, error)
	// pickExportDir opens the native folder picker for .md export, defaulting
	// to the user's Downloads folder; "" = user cancelled.
	pickExportDir func() (string, error)
	// pickOpenPath opens the native file-open dialog; "" = user cancelled.
	pickOpenPath func() (string, error)
}

func NewStore() *Store {
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		cfg = "."
	}
	defaultDir := filepath.Join(cfg, "slite")
	s := &Store{dataDir: defaultDir, defaultDir: defaultDir}

	// Bootstrap: app.json is the single source of truth for where the data
	// lives (default dir when DataDir is ""). First run after an upgrade
	// migrates the legacy anchor (settings.json's dataDir field) into it.
	if d := s.readAppConfigDataDir(); d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				s.dataDir = abs
			} else {
				log.Printf("slite: configured data dir %q unavailable, falling back to default", d)
			}
		}
	}
	s.settings = s.readSettingsFile(filepath.Join(s.dataDir, "settings.json"))

	// Sync the auto-start flag so the settings page reflects reality.
	s.settings.LaunchAtStartup = s.getLaunchAtStartup()

	// Guarantee a concrete default so the UI always shows a real combo.
	if s.settings.Hotkey == "" {
		s.settings.Hotkey = defaultHotkey
	}
	return s
}

// currentSettings returns the in-memory settings (loaded at startup).
// Unexported: not exposed as a binding; the frontend uses LoadSettings.
func (s *Store) currentSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// currentDataDir returns the active data directory.
func (s *Store) currentDataDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dataDir
}

func (s *Store) notesPath() string { return filepath.Join(s.dataDir, "notes.json") }

// notesDir is the per-note layout: one <id>.json file per note. The legacy
// single-file notes.json is migrated into it on first read (see LoadNotes).
func (s *Store) notesDir() string { return filepath.Join(s.dataDir, "notes") }
func (s *Store) notePath(id string) string {
	return filepath.Join(s.notesDir(), id+".json")
}
func (s *Store) settingsPath() string { return filepath.Join(s.dataDir, "settings.json") }

// --- bindings ---

// Ping is a trivial binding used to detect whether the frontend is running
// inside the Wails runtime (native) or in a plain browser (fallback mode).
// It never fails, unlike LoadNotes/LoadSettings which can error on read.
func (s *Store) Ping() string {
	return "pong"
}

// AppVersion returns the user-facing version string for the About section.
func (s *Store) AppVersion() string {
	return appVersion
}

// OpenURL opens a URL in the user's default browser. Implemented per platform
// (Windows ShellExecute / macOS `open`). Used by the About section's links;
// the webview does not follow external links.
func (s *Store) OpenURL(url string) error { return openURL(url) }

// LoadNotes reads all notes from disk (per-note files under notes/), sorting
// by CreatedAt so the list order is stable regardless of directory order. On
// the first read after an upgrade it migrates the legacy single-file
// notes.json into per-note files; a failed migration falls back to serving
// the legacy data and retries on the next call.
func (s *Store) LoadNotes() ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// New layout: notes/ dir. One corrupt note file is skipped, not fatal.
	if entries, err := os.ReadDir(s.notesDir()); err == nil {
		return s.readNotesDir(entries)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read notes dir: %w", err)
	}

	// Legacy single-file layout: migrate on first read.
	data, err := os.ReadFile(s.notesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Note{}, nil // fresh install: nothing to migrate
		}
		return nil, fmt.Errorf("read notes: %w", err)
	}
	var f notesFile
	if err := json.Unmarshal(data, &f); err != nil {
		// Corrupt legacy file: back it up (never destroy user data), then start
		// fresh so the app still boots. The backup keeps the original bytes for
		// manual recovery.
		backup := filepath.Join(s.dataDir, "notes.json.corrupt-"+time.Now().Format("20060102-150405"))
		if rerr := os.WriteFile(backup, data, 0o644); rerr != nil {
			log.Printf("slite: failed to back up corrupt notes file: %v", rerr)
		} else {
			log.Printf("slite: notes.json corrupt (%v); backed up to %s and starting fresh", err, backup)
		}
		return []Note{}, nil
	}
	if f.Notes == nil {
		f.Notes = []Note{}
	}
	if err := s.migrateNotesFile(f.Notes); err != nil {
		// Migration failed: serve the legacy data; the next LoadNotes retries.
		log.Printf("slite: notes migration deferred: %v", err)
		return sortNotesByCreatedAt(f.Notes), nil
	}
	return sortNotesByCreatedAt(f.Notes), nil
}

// readNotesDir reads all per-note files, skipping unreadable or corrupt ones
// (one bad note must not take the rest down). Caller holds s.mu.
func (s *Store) readNotesDir(entries []os.DirEntry) ([]Note, error) {
	notes := make([]Note, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.notesDir(), e.Name()))
		if err != nil {
			log.Printf("slite: skip unreadable note %s: %v", e.Name(), err)
			continue
		}
		var n Note
		if err := json.Unmarshal(data, &n); err != nil {
			log.Printf("slite: skip corrupt note %s: %v", e.Name(), err)
			continue
		}
		notes = append(notes, n)
	}
	return sortNotesByCreatedAt(notes), nil
}

// migrateNotesFile splits the legacy notes.json into per-note files. Each
// note is written individually; only when all succeed is the legacy file
// renamed to a backup. On any failure the partial notes/ dir is removed and
// the legacy file left intact so a later call retries cleanly.
// Caller holds s.mu.
func (s *Store) migrateNotesFile(notes []Note) error {
	if err := os.MkdirAll(s.notesDir(), 0o755); err != nil {
		return fmt.Errorf("create notes dir: %w", err)
	}
	rollback := func() { _ = os.RemoveAll(s.notesDir()) }
	for i := range notes {
		n := &notes[i]
		if n.ID == "" {
			// Defensive: legacy data with a missing id gets a fresh one.
			n.ID = randomID()
		}
		data, err := json.Marshal(n)
		if err != nil {
			rollback()
			return fmt.Errorf("marshal note: %w", err)
		}
		if err := os.WriteFile(s.notePath(n.ID), data, 0o644); err != nil {
			rollback()
			return fmt.Errorf("write note file: %w", err)
		}
	}
	backup := filepath.Join(s.dataDir, "notes.json.migrated-"+time.Now().Format("20060102-150405"))
	if err := os.Rename(s.notesPath(), backup); err != nil {
		rollback()
		return fmt.Errorf("back up legacy notes file: %w", err)
	}
	log.Printf("slite: migrated %d notes to per-note files (backup %s)", len(notes), backup)
	return nil
}

// sortNotesByCreatedAt orders notes oldest-first (new notes at the end),
// matching the pre-migration list order.
func sortNotesByCreatedAt(notes []Note) []Note {
	sort.SliceStable(notes, func(i, j int) bool {
		return notes[i].CreatedAt < notes[j].CreatedAt
	})
	return notes
}

// randomID returns a 32-hex-char id (defensive fallback for legacy notes
// that lack an id; the frontend normally generates UUIDs).
func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("n%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// SaveNote persists a single note atomically. Per-note files mean editing one
// note rewrites only that file, never the whole dataset.
func (s *Store) SaveNote(note Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if note.ID == "" {
		return fmt.Errorf("note id is empty")
	}
	if err := os.MkdirAll(s.notesDir(), 0o755); err != nil {
		return fmt.Errorf("create notes dir: %w", err)
	}
	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal note: %w", err)
	}
	path := s.notePath(note.ID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write note temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename note: %w", err)
	}
	return nil
}

// DeleteNote removes a note's file. Removing a missing note is not an error
// (idempotent, so a stale delete after a reload is harmless).
func (s *Store) DeleteNote(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.notePath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

// LoadSettings reads persisted settings (falling back to defaults).
func (s *Store) LoadSettings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}

// SaveSettings persists settings and applies window-level side effects
// (always-on-top, window opacity, auto-start). The data directory is managed
// exclusively by MoveDataDir/UseDataDir; window bounds exclusively by
// SaveWindowBounds.
func (s *Store) SaveSettings(settings Settings) error {
	if settings.Theme == "" {
		settings.Theme = "system"
	}
	// Defensive clamp for the opacity slider.
	if settings.Opacity < 0.3 {
		settings.Opacity = 1
	} else if settings.Opacity > 1 {
		settings.Opacity = 1
	}
	s.mu.Lock()
	prev := s.settings
	// Window bounds are owned by the Go side and updated asynchronously on
	// move/resize; a stale frontend snapshot (loaded at boot) must never
	// clobber them.
	settings.WindowX = prev.WindowX
	settings.WindowY = prev.WindowY
	settings.WindowWidth = prev.WindowWidth
	settings.WindowHeight = prev.WindowHeight
	s.settings = settings
	s.mu.Unlock()

	if err := s.writeJSONAtomic(s.settingsPath(), s.settings); err != nil {
		s.mu.Lock()
		s.settings = prev
		s.mu.Unlock()
		return err
	}

	if s.settings.LaunchAtStartup != prev.LaunchAtStartup {
		if err := s.setLaunchAtStartup(s.settings.LaunchAtStartup); err != nil {
			return err
		}
	}
	if mainWindow != nil {
		mainWindow.SetAlwaysOnTop(settings.AlwaysOnTop)
		if !opacityOverride {
			if err := setWindowOpacity(settings.Opacity); err != nil {
				log.Printf("slite: set opacity: %v", err)
			}
		}
	}
	return nil
}

// SetWindowOpacityOverride temporarily forces the window fully opaque while
// an app-modal overlay (settings panel, theme picker) is open, so the
// translucent backdrop does not muddy the overlay UI. on=false restores the
// persisted opacity. No-op in browser fallback (the frontend never calls it).
func (s *Store) SetWindowOpacityOverride(on bool) {
	setOpacityOverride(on)
}

// SetHotkey re-registers the global toggle hotkey (no-op in browser fallback,
// where hotkeyReconfigure is nil) without persisting; the frontend then calls
// SaveSettings with the new combo. On any failure the previous binding is left
// untouched.
func (s *Store) SetHotkey(combo string) error {
	var err error
	combo, err = windowutil.NormalizeHotkey(combo)
	if err != nil {
		return err
	}
	if s.hotkeyReconfigure == nil {
		// Browser fallback mode: nothing to register.
		return nil
	}
	return s.hotkeyReconfigure(combo)
}

// ValidateDataDir runs the pre-checks for a candidate data directory in
// "adopt" mode (empty, or containing only slite-owned files). The settings
// panel calls it for a fast failure before the real operation.
func (s *Store) ValidateDataDir(path string) error {
	return s.validateDataDir(path, false)
}

// MoveDataDir migrates the data (settings.json + notes) into target, points
// app.json at it, then removes the old files. The target must be empty — this
// is a move, never an overwrite. Write-then-delete ordering means a crash
// mid-move leaves the old directory intact and app.json untouched, so no
// data is lost.
func (s *Store) MoveDataDir(path string) error {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if err := s.validateDataDir(abs, true); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	oldDir := s.dataDir

	copied, err := s.copyDataDirContents(oldDir, abs)
	if err != nil {
		s.removeCopied(abs, copied)
		return err
	}

	// Point the bootstrap at the new location, then drop the old files.
	// Migrating back into the default dir stores the canonical "" form.
	s.dataDir = abs
	pointer := abs
	if strings.EqualFold(filepath.Clean(abs), filepath.Clean(s.defaultDir)) {
		pointer = ""
	}
	if err := s.writeAppConfig(AppConfig{Version: appConfigVersion, DataDir: pointer}); err != nil {
		s.dataDir = oldDir
		s.removeCopied(abs, copied)
		return err
	}
	for _, name := range copied {
		_ = os.RemoveAll(filepath.Join(oldDir, name))
	}
	return nil
}

// UseDataDir adopts an existing slite data directory (or an empty folder as a
// fresh one) without copying or deleting anything: app.json points at it and
// preferences reload from it. This is how a reinstall / new machine reconnects
// to previously created data.
func (s *Store) UseDataDir(path string) error {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if err := s.validateDataDir(abs, false); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	oldDir, oldSettings := s.dataDir, s.settings
	s.dataDir = abs
	s.settings = s.readSettingsFile(filepath.Join(abs, "settings.json"))
	pointer := abs
	if strings.EqualFold(filepath.Clean(abs), filepath.Clean(s.defaultDir)) {
		pointer = ""
	}
	if err := s.writeAppConfig(AppConfig{Version: appConfigVersion, DataDir: pointer}); err != nil {
		s.dataDir, s.settings = oldDir, oldSettings
		return err
	}
	return nil
}

// copyDataDirContents copies the data artifacts that follow a move
// (settings.json + whichever note layout exists) into dst. notes.json is the
// legacy single-file layout; notes/ the per-note layout that LoadNotes
// migrates to. Whichever exists is carried over, so one function covers both
// layouts. Returns the artifact names actually copied (for rollback/cleanup).
func (s *Store) copyDataDirContents(src, dst string) ([]string, error) {
	var copied []string
	for _, name := range []string{"settings.json", "notes.json", "notes"} {
		srcPath := filepath.Join(src, name)
		info, err := os.Stat(srcPath)
		if err != nil {
			continue // not present — nothing to carry over
		}
		if info.IsDir() {
			entries, err := os.ReadDir(srcPath)
			if err != nil {
				return copied, fmt.Errorf("read %s: %w", name, err)
			}
			if err := os.MkdirAll(filepath.Join(dst, name), 0o755); err != nil {
				return copied, fmt.Errorf("create %s: %w", name, err)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				data, err := os.ReadFile(filepath.Join(srcPath, e.Name()))
				if err != nil {
					return copied, fmt.Errorf("read %s: %w", filepath.Join(name, e.Name()), err)
				}
				if err := os.WriteFile(filepath.Join(dst, name, e.Name()), data, 0o644); err != nil {
					return copied, fmt.Errorf("copy %s: %w", filepath.Join(name, e.Name()), err)
				}
			}
			copied = append(copied, name)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			return copied, fmt.Errorf("copy %s: %w", name, err)
		}
		copied = append(copied, name)
	}
	return copied, nil
}

// removeCopied deletes artifacts that were written into dir during a
// migration, for rollback or post-move cleanup.
func (s *Store) removeCopied(dir string, names []string) {
	for _, n := range names {
		_ = os.RemoveAll(filepath.Join(dir, n))
	}
}

// writeAnchorSettings is gone: the bootstrap pointer now lives in app.json
// (see writeAppConfig). No legacy anchor is written or read anymore beyond
// the one-time migration in readAppConfigDataDir.

// SuspendHotkey temporarily unregisters the global toggle hotkey while the
// user records a new combo in the settings panel, so pressing the old combo
// mid-recording cannot toggle the window. No-op in browser fallback.
func (s *Store) SuspendHotkey() error {
	if s.hotkeySuspend == nil {
		return nil
	}
	return s.hotkeySuspend()
}

// ResumeHotkey restores the toggle hotkey after SuspendHotkey (re-registering
// whatever combo is current, which may be a newly configured one).
func (s *Store) ResumeHotkey() error {
	if s.hotkeyResume == nil {
		return nil
	}
	return s.hotkeyResume()
}

// ChooseDataDir opens the native folder picker and returns the selected path,
// or "" when the user cancels. Error in browser fallback mode.
func (s *Store) ChooseDataDir() (string, error) {
	if s.pickDir == nil {
		return "", fmt.Errorf("folder picker unavailable")
	}
	return s.pickDir()
}

// SaveWindowBounds persists the window's screen position and size (physical
// pixels). Called debounced from the Go side on window move/resize; the
// frontend never calls it. Only the four bounds fields are touched, so the
// always-on-top / opacity / autostart side effects of SaveSettings do not
// run on every drag.
func (s *Store) SaveWindowBounds(x, y, w, h int) error {
	s.mu.Lock()
	prev := s.settings
	s.settings.WindowX, s.settings.WindowY = x, y
	s.settings.WindowWidth, s.settings.WindowHeight = w, h
	s.mu.Unlock()

	if err := s.writeJSONAtomic(s.settingsPath(), s.settings); err != nil {
		s.mu.Lock()
		s.settings = prev
		s.mu.Unlock()
		return err
	}
	return nil
}

// MarkdownFile is one note's display name + markdown body, used for bulk
// .md export.
type MarkdownFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// OpenMarkdownDialog shows the native open dialog (markdown/text filter) and
// returns the chosen file's content ("" when the user cancels or the file is
// empty).
func (s *Store) OpenMarkdownDialog() (string, error) {
	if s.pickOpenPath == nil {
		return "", fmt.Errorf("open dialog unavailable")
	}
	path, err := s.pickOpenPath()
	if err != nil || path == "" {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// ExportAllMarkdown writes every note as its own .md file into a folder the
// user picks (defaulting to Downloads). Single-note export passes a
// one-element slice. Returns the number of files written (0 when the user
// cancels). Colliding names get a numeric suffix; invalid filename
// characters are replaced so the export can never fail on the target
// filesystem.
func (s *Store) ExportAllMarkdown(files []MarkdownFile) (int, error) {
	if s.pickExportDir == nil {
		return 0, fmt.Errorf("folder picker unavailable")
	}
	dir, err := s.pickExportDir()
	if err != nil {
		return 0, err
	}
	if dir == "" {
		return 0, nil // cancelled
	}
	used := map[string]bool{}
	written := 0
	for _, f := range files {
		name := sanitizeFileName(f.Name)
		final := name + ".md"
		for n := 2; used[strings.ToLower(final)]; n++ {
			final = fmt.Sprintf("%s (%d).md", name, n)
		}
		used[strings.ToLower(final)] = true
		if err := os.WriteFile(filepath.Join(dir, final), []byte(f.Content), 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", final, err)
		}
		written++
	}
	return written, nil
}

// sanitizeFileName strips characters that are invalid in Windows file names
// and trims trailing dots/spaces (which Windows treats as separators). Empty
// input falls back to "Untitled".
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		return r
	}, name)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return "Untitled"
	}
	return name
}

// OpenDataDir reveals the active data directory in the file manager
// (Explorer on Windows, Finder on macOS). No-op in browser fallback mode.
func (s *Store) OpenDataDir() error {
	return openDataDir(s.currentDataDir())
}

// CurrentDataDir returns the active data directory path (display in settings).
func (s *Store) CurrentDataDir() string {
	return s.currentDataDir()
}

// --- internal ---

// validateDataDir checks a candidate data directory. forMove=true requires an
// empty target (move never overwrites); forMove=false (adopt) allows an empty
// folder or one holding only slite-owned files — including the legacy
// notes.json layout, which LoadNotes migrates on next read. Checks:
//   - resolves to an absolute path
//   - is not the currently active data directory
//   - exists and is a directory
//   - is writable (probe file create+delete)
//   - content per the mode above
func (s *Store) validateDataDir(path string, forMove bool) error {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if strings.EqualFold(filepath.Clean(abs), filepath.Clean(s.currentDataDir())) {
		return fmt.Errorf("this is already the active data directory")
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", abs)
		}
		return fmt.Errorf("cannot access %s: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", abs)
	}
	// Writable probe.
	probe := filepath.Join(abs, ".slite-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	_ = os.Remove(probe)
	// Content check per mode.
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		// Bootstrap residue (.slite-write-test probe, app.json pointer) is
		// tolerated in both modes; it is not data.
		if name == ".slite-write-test" || name == "app.json" {
			continue
		}
		if forMove {
			return fmt.Errorf("target directory is not empty")
		}
		switch name {
		case "settings.json", "notes.json", "notes.json.tmp", "log.txt":
			continue
		case "notes":
			if !e.IsDir() {
				return fmt.Errorf("unexpected file %q in directory", e.Name())
			}
			continue
		default:
			// Backup artifacts slite itself created (corrupt/migrated legacy
			// notes.json) are tolerated, not foreign content.
			if strings.HasPrefix(name, "notes.json.corrupt-") || strings.HasPrefix(name, "notes.json.migrated-") {
				continue
			}
			return fmt.Errorf("directory contains files that don't belong to slite (%q)", e.Name())
		}
	}
	return nil
}

// appConfigPath is the bootstrap pointer's fixed location. It stays in the
// default dir even when the data lives elsewhere, and survives uninstall
// because the NSIS uninstaller keeps %APPDATA%\slite.
func (s *Store) appConfigPath() string { return filepath.Join(s.defaultDir, "app.json") }

// readAppConfigDataDir returns the configured data directory from app.json
// ("" = default). On the first run after an upgrade from the legacy layout it
// migrates the old anchor — settings.json's dataDir field, which used to
// double as the bootstrap pointer — into app.json so the boot path is
// uniform from then on. Failures to write the migrated/default config are
// non-fatal: a missing app.json falls back to the default directory.
func (s *Store) readAppConfigDataDir() string {
	if data, err := os.ReadFile(s.appConfigPath()); err == nil {
		var cfg AppConfig
		if json.Unmarshal(data, &cfg) == nil {
			return cfg.DataDir
		}
	}
	// Legacy migration: the pre-app.json settings.json carried dataDir.
	legacy := struct {
		DataDir string `json:"dataDir"`
	}{}
	if data, err := os.ReadFile(filepath.Join(s.defaultDir, "settings.json")); err == nil {
		if json.Unmarshal(data, &legacy) == nil && legacy.DataDir != "" {
			pointer := legacy.DataDir
			// A legacy pointer at the default dir itself is stored in the
			// canonical "" form, matching MoveDataDir/UseDataDir.
			if abs, err := filepath.Abs(legacy.DataDir); err == nil && strings.EqualFold(filepath.Clean(abs), filepath.Clean(s.defaultDir)) {
				pointer = ""
			}
			_ = s.writeAppConfig(AppConfig{Version: appConfigVersion, DataDir: pointer})
			return pointer
		}
	}
	// Default layout (or no legacy anchor): record it explicitly so every boot
	// reads the same single source. Never fatal — a read-only AppData degrades
	// to an implicit default.
	_ = s.writeAppConfig(AppConfig{Version: appConfigVersion})
	return ""
}

// writeAppConfig persists the bootstrap pointer atomically. Called with s.mu
// held by migration paths; NewStore calls it before the store is shared.
func (s *Store) writeAppConfig(cfg AppConfig) error {
	if err := os.MkdirAll(s.defaultDir, 0o755); err != nil {
		return fmt.Errorf("create default data dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal app config: %w", err)
	}
	path := s.appConfigPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write app config temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename app config: %w", err)
	}
	return nil
}

// setLaunchAtStartup and getLaunchAtStartup are implemented per platform
// (Windows HKCU Run key / macOS Wails Autostart — see store_platform_*.go).

func (s *Store) readSettingsFile(path string) Settings {
	settings := Settings{Theme: "system"}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings
	}
	if settings.Theme == "" {
		settings.Theme = "system"
	}
	return settings
}

func (s *Store) writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
