//go:build windows

package main

// autostartIdentifier is the HKCU Run value name used by app.Autostart. It
// stays the plain "slite-note" (not a reverse-DNS id) so enabling auto-start
// overwrites the value earlier releases wrote instead of registering a second
// login item next to it.
const autostartIdentifier = "slite-note"
