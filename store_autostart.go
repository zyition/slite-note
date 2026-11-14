package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// setLaunchAtStartup registers/removes the OS login item through Wails'
// cross-platform Autostart manager (SMAppService on macOS 13+, LaunchAgent
// plist fallback for unbundled binaries, the HKCU Run key on Windows).
// Arguments carry --silent so a login start never pops the window.
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

// getLaunchAtStartup reports whether a login item for this executable exists.
// The manager compares the registered command with the running binary, so a
// stale entry pointing at an old path (e.g. the pre-rename slite.exe) reads as
// disabled instead of claiming an auto-start that never happens.
func (s *Store) getLaunchAtStartup() bool {
	if app == nil {
		return false
	}
	ok, err := app.Autostart.IsEnabled()
	return err == nil && ok
}
