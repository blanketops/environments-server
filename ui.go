package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// uiDir returns the directory holding the built UI (vite's dist/), or "" when
// there is none. UI_DIR wins; otherwise KO_DATA_PATH, which ko sets to the
// image's copy of server/kodata (ko.yml copies dist/ there before building). A
// directory without index.html is treated as absent, so a local `go run`
// without a UI build serves the API only.
func uiDir() string {
	for _, dir := range []string{os.Getenv("UI_DIR"), os.Getenv("KO_DATA_PATH")} {
		if dir == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !st.IsDir() {
			return dir
		}
	}
	return ""
}

// uiHandler serves the built UI from dir. A path naming a file is served as
// that file; any other path gets index.html so React Router can resolve the
// client-side route. Unknown /api/ paths stay a JSON 404 instead of turning
// into an HTML page with status 200.
func uiHandler(dir string) http.Handler {
	root := http.Dir(dir)
	files := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean("/" + r.URL.Path)
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			writeErr(w, http.StatusNotFound, "no such API route: "+p, "NotFound")
			return
		}
		if p != "/" {
			if f, err := root.Open(p); err == nil {
				st, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !st.IsDir() {
					files.ServeHTTP(w, r)
					return
				}
			}
		}
		index, err := root.Open("/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = index.Close() }()
		st, err := index.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", st.ModTime(), index)
	})
}
