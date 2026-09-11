package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Attachment responses must never be cacheable: WKWebView once cached a
// truncated response under its heuristic caching and kept serving the broken
// image forever (the bytes on disk were fine). no-store makes every load hit
// the file, so a bad response cannot outlive its request.
func TestServeAttachmentSetsNoStore(t *testing.T) {
	s := newTestStore(t)
	dir := filepath.Join(s.dataDir, "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "abc123.png"), []byte("pngbytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	serveAttachment(s, rec, httptest.NewRequest(http.MethodGet, "/attachments/abc123.png", nil))

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != "pngbytes" {
		t.Fatalf("unexpected response: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// Path traversal stays blocked (regression guard next to the header change).
func TestServeAttachmentRejectsTraversal(t *testing.T) {
	s := newTestStore(t)
	rec := httptest.NewRecorder()
	serveAttachment(s, rec, httptest.NewRequest(http.MethodGet, "/attachments/..%2Fsettings.json", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal escaped: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

// Tray labels resolve per language with English as the fallback for "" (follow
// the OS) and any unknown value — the tray is built before the webview can
// push the resolved locale.
func TestTrayLabelsFor(t *testing.T) {
	en := trayLabelsFor("en")
	if en.showHide != "Show/Hide" || en.settings != "Settings..." || en.quit != "Quit" {
		t.Fatalf("unexpected en labels: %+v", en)
	}
	zh := trayLabelsFor("zh-CN")
	if zh.showHide == en.showHide || zh.quit == en.quit {
		t.Fatalf("zh-CN labels must differ from en: %+v", zh)
	}
	for _, lang := range []string{"", "system", "fr-FR"} {
		if got := trayLabelsFor(lang); got != en {
			t.Fatalf("fallback for %q = %+v, want en %+v", lang, got, en)
		}
	}
}
