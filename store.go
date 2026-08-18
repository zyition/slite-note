package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"
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
	Theme           string `json:"theme"`           // "yellow" | "gray" | "dark"
	AlwaysOnTop     bool   `json:"alwaysOnTop"`
	Hotkey          string `json:"hotkey"`          // global toggle accelerator, e.g. "Alt+Shift+S"
	LaunchAtStartup bool   `json:"launchAtStartup"` // Windows Run key (HKCU)
	DataDir         string `json:"dataDir"`         // "" = default os.UserConfigDir()/slite
}

const notesFileVersion = 1

type notesFile struct {
	Version int    `json:"version"`
	Notes   []Note `json:"notes"`
}

// runKeyPath is the HKCU auto-start location for the current user.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// Store is the persistence service bound to the frontend. It owns the data
// directory (os.UserConfigDir()/slite by default, configurable) and
// reads/writes notes.json and settings.json. The frontend calls these methods
// via generated bindings; in pure-browser fallback mode the frontend uses
// localStorage instead.
type Store struct {
	mu       sync.Mutex
	dataDir  string
	settings Settings

	// hotkeyReconfigure is injected by main.go; it re-registers the global
	// toggle hotkey without disturbing the existing binding on failure.
	hotkeyReconfigure func(combo string) error
	// hotkeySuspend/hotkeyResume temporarily unregister/restore the global
	// toggle hotkey while the user records a new combo in the settings panel.
	hotkeySuspend func() error
	hotkeyResume  func() error
	// pickDir opens the native folder picker (Windows).
	pickDir func() (string, error)
}

func NewStore() *Store {
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		cfg = "."
	}
	defaultDir := filepath.Join(cfg, "slite")
	s := &Store{dataDir: defaultDir}
	s.settings = s.readSettingsFile(filepath.Join(defaultDir, "settings.json"))

	// Honor a persisted custom data directory if it still exists and is a dir.
	if d := s.settings.DataDir; d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				s.dataDir = abs
				if st := s.readSettingsFile(filepath.Join(abs, "settings.json")); st.Theme != "" {
					s.settings = st
				}
			} else {
				log.Printf("slite: configured data dir %q unavailable, falling back to default", d)
			}
		}
	}
	s.settings.DataDir = s.dataDir

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

func (s *Store) notesPath() string   { return filepath.Join(s.dataDir, "notes.json") }
func (s *Store) settingsPath() string { return filepath.Join(s.dataDir, "settings.json") }

// --- bindings ---

// Ping is a trivial binding used to detect whether the frontend is running
// inside the Wails runtime (native) or in a plain browser (fallback mode).
// It never fails, unlike LoadNotes/LoadSettings which can error on read.
func (s *Store) Ping() string {
	return "pong"
}

// LoadNotes reads all notes from disk. Returns an empty slice if the file does
// not exist yet.
func (s *Store) LoadNotes() ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.notesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Note{}, nil
		}
		return nil, fmt.Errorf("read notes: %w", err)
	}
	var f notesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse notes: %w", err)
	}
	if f.Notes == nil {
		f.Notes = []Note{}
	}
	return f.Notes, nil
}

// SaveNotes persists the full note list atomically. Note deletion is handled
// on the frontend (remove from list, then save the full list).
func (s *Store) SaveNotes(notes []Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := notesFile{Version: notesFileVersion, Notes: notes}
	return s.writeJSONAtomic(s.notesPath(), f)
}

// LoadSettings reads persisted settings (falling back to defaults).
func (s *Store) LoadSettings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}

// SaveSettings persists settings and applies window-level side effects
// (always-on-top, auto-start). DataDir is managed exclusively by SetDataDir.
func (s *Store) SaveSettings(settings Settings) error {
	if settings.Theme == "" {
		settings.Theme = "yellow"
	}
	s.mu.Lock()
	prev := s.settings
	s.settings = settings
	s.settings.DataDir = s.dataDir // migration owns this field
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
	}
	return nil
}

// SetHotkey re-registers the global toggle hotkey (no-op in browser fallback,
// where hotkeyReconfigure is nil) without persisting; the frontend then calls
// SaveSettings with the new combo. On any failure the previous binding is left
// untouched.
func (s *Store) SetHotkey(combo string) error {
	combo = strings.TrimSpace(combo)
	if combo == "" {
		return fmt.Errorf("hotkey must not be empty")
	}
	if s.hotkeyReconfigure == nil {
		// Browser fallback mode: nothing to register.
		return nil
	}
	return s.hotkeyReconfigure(combo)
}

// ValidateDataDir runs the pre-migration checks for a candidate data directory.
// It returns a non-nil error describing the first failed check. Checks:
//   - resolves to an absolute path
//   - is not the currently active data directory
//   - exists and is a directory
//   - is writable (probe file create+delete)
//   - contains only slite-owned files (notes.json, settings.json, log.txt) or nothing
func (s *Store) ValidateDataDir(path string) error {
	return s.validateDataDir(path)
}

// SetDataDir validates the target, migrates notes.json (if any), switches the
// active directory, persists settings, then removes the old slite files.
func (s *Store) SetDataDir(path string) error {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if err := s.validateDataDir(abs); err != nil {
		return err
	}

	s.mu.Lock()
	oldDir := s.dataDir

	// Copy notes if present. Write to a temp name first, then rename, to avoid
	// half-written files if the copy is interrupted.
	copiedNotes := false
	if data, err := os.ReadFile(filepath.Join(oldDir, "notes.json")); err == nil {
		tmp := filepath.Join(abs, "notes.json.tmp")
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			s.mu.Unlock()
			return fmt.Errorf("copy notes to new dir: %w", err)
		}
		if err := os.Rename(tmp, filepath.Join(abs, "notes.json")); err != nil {
			os.Remove(tmp)
			s.mu.Unlock()
			return fmt.Errorf("copy notes to new dir: %w", err)
		}
		copiedNotes = true
	}

	s.dataDir = abs
	s.settings.DataDir = abs
	if err := s.writeJSONAtomic(s.settingsPath(), s.settings); err != nil {
		// Roll back the directory switch and remove the copied notes.
		s.dataDir = oldDir
		s.settings.DataDir = oldDir
		if copiedNotes {
			os.Remove(filepath.Join(abs, "notes.json"))
		}
		s.mu.Unlock()
		return fmt.Errorf("write settings in new dir: %w", err)
	}
	s.mu.Unlock()

	// Move semantics: remove the old slite files (best effort).
	os.Remove(filepath.Join(oldDir, "notes.json"))
	os.Remove(filepath.Join(oldDir, "settings.json"))
	return nil
}

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

// OpenDataDir reveals the active data directory in Explorer (no-op in browser
// fallback mode).
func (s *Store) OpenDataDir() error {
	dir := s.currentDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	// explorer.exe resolves the directory and shows it; single argument so
	// spaces in the path are safe.
	return exec.Command("explorer.exe", dir).Start()
}

// CurrentDataDir returns the active data directory path (display in settings).
func (s *Store) CurrentDataDir() string {
	return s.currentDataDir()
}

// --- internal ---

func (s *Store) validateDataDir(path string) error {
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
	// Only slite-owned files may be present.
	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}
	for _, e := range entries {
		switch strings.ToLower(e.Name()) {
		case "notes.json", "notes.json.tmp", "settings.json", "log.txt", ".slite-write-test":
			continue
		default:
			return fmt.Errorf("directory is not empty (found %q)", e.Name())
		}
	}
	return nil
}

// setLaunchAtStartup adds/removes the HKCU Run entry for this executable.
func (s *Store) setLaunchAtStartup(enabled bool) error {
	if enabled {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
		k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("open run key: %w", err)
		}
		defer k.Close()
		if err := k.SetStringValue("slite", `"`+exe+`"`); err != nil {
			return fmt.Errorf("write run key: %w", err)
		}
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer k.Close()
	if err := k.DeleteValue("slite"); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete run key: %w", err)
	}
	return nil
}

// getLaunchAtStartup reports whether the HKCU Run entry currently exists.
func (s *Store) getLaunchAtStartup() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue("slite")
	return err == nil
}

func (s *Store) readSettingsFile(path string) Settings {
	settings := Settings{Theme: "yellow"}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings
	}
	if settings.Theme == "" {
		settings.Theme = "yellow"
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
