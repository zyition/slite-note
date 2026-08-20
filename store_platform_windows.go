//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// runKeyPath is the HKCU auto-start location for the current user.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

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

// openURL opens a URL in the user's default browser (Windows ShellExecute).
func openURL(url string) error {
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

// openDataDir reveals the directory in Explorer; single argument so spaces
// in the path are safe.
func openDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	return exec.Command("explorer.exe", dir).Start()
}
