package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeUI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestUIHandler(t *testing.T) {
	h := uiHandler(writeUI(t))

	cases := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{"/", http.StatusOK, "<html>app</html>"},
		{"/builds", http.StatusOK, "<html>app</html>"},
		{"/builds/new", http.StatusOK, "<html>app</html>"},
		{"/assets", http.StatusOK, "<html>app</html>"},
		{"/assets/app.js", http.StatusOK, "console.log(1)"},
		{"/api/v2/nothing", http.StatusNotFound, "no such API route"},
		{"/api", http.StatusNotFound, "no such API route"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestUIHandlerWithoutIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	uiHandler(t.TempDir()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/builds", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUIDir(t *testing.T) {
	ui := writeUI(t)
	empty := t.TempDir()

	cases := []struct {
		name, uiDir, koData, want string
	}{
		{"none", "", "", ""},
		{"ui dir", ui, "", ui},
		{"ko data", "", ui, ui},
		{"ui dir wins", ui, empty, ui},
		{"no index falls through", empty, ui, ui},
		{"no index anywhere", empty, empty, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("UI_DIR", tc.uiDir)
			t.Setenv("KO_DATA_PATH", tc.koData)
			if got := uiDir(); got != tc.want {
				t.Fatalf("uiDir() = %q, want %q", got, tc.want)
			}
		})
	}
}
