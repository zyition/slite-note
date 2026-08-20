//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// autostartIdentifier is the launchd Label / SMAppService key. Wails derives
// a slug from the app name by default, but an explicit reverse-DNS identifier
// is the mac convention and survives renames.
const autostartIdentifier = "com.zyition.slite-note"

// setLaunchAtStartup registers/removes login auto-start through Wails'
// cross-platform Autostart manager (SMAppService on macOS 13+, LaunchAgent
// plist fallback for unbundled binaries). Arguments carry --silent so a
// login start never pops the window — same semantics as the Windows Run key.
func (s *Store) setLaunchAtStartup(enabled bool) error {
	if app == nil {
		return fmt.Errorf("application not initialised")
	}
	if enabled {
		return app.Autostart.EnableWithOptions(application.AutostartOptions{
			Identifier: autostartIdentifier,
			Arguments:  []string{"--silent"},
		})
	}
	return app.Autostart.Disable()
}

// getLaunchAtStartup reports whether an autostart registration exists.
func (s *Store) getLaunchAtStartup() bool {
	if app == nil {
		return false
	}
	ok, err := app.Autostart.IsEnabled()
	return err == nil && ok
}

// openURL opens a URL in the user's default browser via `open` (the mac way).
func openURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}
	return exec.Command("open", url).Start()
}

// openDataDir reveals the directory in Finder.
func openDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	return exec.Command("open", dir).Start()
}
