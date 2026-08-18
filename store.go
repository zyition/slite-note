package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	Theme       string `json:"theme"`       // "yellow" | "gray" | "dark"
	AlwaysOnTop bool   `json:"alwaysOnTop"`
}

const notesFileVersion = 1

type notesFile struct {
	Version int    `json:"version"`
	Notes   []Note `json:"notes"`
}

// Store is the persistence service bound to the frontend. It owns the data
// directory (os.UserConfigDir()/slite) and reads/writes notes.json and
// settings.json. The frontend calls these methods via generated bindings; in
// pure-browser fallback mode the frontend uses localStorage instead.
type Store struct {
	mu       sync.Mutex
	dataDir  string
	settings Settings
}

func NewStore() *Store {
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		cfg = "."
	}
	s := &Store{dataDir: filepath.Join(cfg, "slite")}
	s.settings = s.loadSettingsFromDisk()
	return s
}

// currentSettings returns the in-memory settings (loaded at startup).
// Unexported: not exposed as a binding; the frontend uses LoadSettings.
func (s *Store) currentSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Store) notesPath() string  { return filepath.Join(s.dataDir, "notes.json") }
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

// SaveNotes persists the full note list atomically.
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

// SaveSettings persists settings and applies AlwaysOnTop to the window.
func (s *Store) SaveSettings(settings Settings) error {
	if settings.Theme == "" {
		settings.Theme = "yellow"
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	if err := s.writeJSONAtomic(s.settingsPath(), settings); err != nil {
		return err
	}
	if mainWindow != nil {
		mainWindow.SetAlwaysOnTop(settings.AlwaysOnTop)
	}
	return nil
}

// --- internal ---

func (s *Store) loadSettingsFromDisk() Settings {
	settings := Settings{Theme: "yellow", AlwaysOnTop: false}
	data, err := os.ReadFile(s.settingsPath())
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
