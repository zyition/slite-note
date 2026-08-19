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
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
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
	Theme           string  `json:"theme"` // "system" | "yellow" | "gray" | "dark"
	AlwaysOnTop     bool    `json:"alwaysOnTop"`
	Hotkey          string  `json:"hotkey"`          // global toggle accelerator, e.g. "Alt+Shift+S"
	LaunchAtStartup bool    `json:"launchAtStartup"` // Windows Run key (HKCU)
	DataDir         string  `json:"dataDir"`         // "" = default os.UserConfigDir()/slite
	Opacity         float64 `json:"opacity"`         // window opacity 0.3–1.0, 1 = opaque

	// Window bounds in physical pixels, persisted (debounced) on move/resize
	// so the window reopens where the user left it. 0 = never saved yet.
	// Owned by the Go side (SaveWindowBounds); the frontend must not set them.
	WindowX      int `json:"windowX,omitempty"`
	WindowY      int `json:"windowY,omitempty"`
	WindowWidth  int `json:"windowWidth,omitempty"`
	WindowHeight int `json:"windowHeight,omitempty"`
}

// appVersion is the user-facing version shown in the About section. The
// release CI (release.yml) overwrites this constant from the git tag before
// building, so it only serves as the fallback for local / non-release
// builds; keep it at the last released version.
const appVersion = "0.2.0"

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
	mu      sync.Mutex
	dataDir string
	// defaultDir is os.UserConfigDir()/slite — the directory NewStore boots
	// into unless settings.json there points at a custom DataDir. Its
	// settings.json doubles as the anchor that lets a restart rediscover a
	// custom data directory, so SetDataDir must keep it in sync (see
	// writeAnchorSettings).
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

func (s *Store) notesPath() string    { return filepath.Join(s.dataDir, "notes.json") }
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

// OpenURL opens a URL in the user's default browser (Windows ShellExecute).
// Used by the About section's links; WebView2 does not follow external links.
func (s *Store) OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}
	shell := windows.NewLazySystemDLL("shell32.dll")
	proc := shell.NewProc("ShellExecuteW")
	u, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	op, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(u)), 0, 0, 1) // SW_SHOWNORMAL
	if r <= 32 {
		return fmt.Errorf("ShellExecute failed: %d", r)
	}
	return nil
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
		// Corrupt notes file: back it up (never destroy user data), then start
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
// (always-on-top, window opacity, auto-start). DataDir is managed
// exclusively by SetDataDir; window bounds exclusively by SaveWindowBounds.
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
	// Keep the default dir's settings.json in sync as the DataDir anchor so a
	// restart (NewStore reads only the default dir first) can rediscover this
	// custom directory. A failure here rolls the migration back — the old dir
	// must stay untouched rather than half-migrated.
	if err := s.writeAnchorSettings(); err != nil {
		s.dataDir = oldDir
		s.settings.DataDir = oldDir
		if copiedNotes {
			os.Remove(filepath.Join(abs, "notes.json"))
		}
		s.mu.Unlock()
		return fmt.Errorf("write anchor settings: %w", err)
	}
	s.mu.Unlock()

	// Move semantics: remove the old slite files (best effort). The default
	// dir's settings.json is exempt — it is the DataDir anchor above.
	os.Remove(filepath.Join(oldDir, "notes.json"))
	if !strings.EqualFold(filepath.Clean(oldDir), filepath.Clean(s.defaultDir)) {
		os.Remove(filepath.Join(oldDir, "settings.json"))
	}
	return nil
}

// writeAnchorSettings persists the current settings (whose DataDir points at
// the active directory) into the default directory's settings.json. NewStore
// reads that file first at startup to rediscover a custom data directory, so
// this anchor is what makes a custom DataDir survive restarts and repeated
// migrations (default → A → B must end with an anchor pointing at B). No-op
// when the active directory already is the default one — it is written by
// writeJSONAtomic via settingsPath. Must be called with s.mu held.
func (s *Store) writeAnchorSettings() error {
	if strings.EqualFold(filepath.Clean(s.dataDir), filepath.Clean(s.defaultDir)) {
		return nil
	}
	if err := os.MkdirAll(s.defaultDir, 0o755); err != nil {
		return fmt.Errorf("create default data dir: %w", err)
	}
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal anchor settings: %w", err)
	}
	path := filepath.Join(s.defaultDir, "settings.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write anchor temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename anchor: %w", err)
	}
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
		// --silent: auto-start must not pop the window; the tray/hotkey
		// summon it when needed.
		if err := k.SetStringValue("slite-note", `"`+exe+`" --silent`); err != nil {
			return fmt.Errorf("write run key: %w", err)
		}
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer k.Close()
	// Remove the current value name plus the pre-rename legacy "slite" entry
	// (written by releases before the slite-note rename), so the switch can
	// never report a stale on-state.
	if err := k.DeleteValue("slite-note"); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete run key: %w", err)
	}
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
	// "slite-note" is the current value name; "slite" was written by releases
	// before the rename and is still honored so existing users keep their
	// auto-start state across the upgrade.
	if _, _, err = k.GetStringValue("slite-note"); err == nil {
		return true
	}
	_, _, err = k.GetStringValue("slite")
	return err == nil
}

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
