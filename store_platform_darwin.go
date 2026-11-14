//go:build darwin

package main

// autostartIdentifier is the launchd Label / SMAppService key. Wails derives a
// slug from the app name by default, but an explicit reverse-DNS identifier is
// the mac convention and survives renames.
const autostartIdentifier = "com.zyition.slite-note"
