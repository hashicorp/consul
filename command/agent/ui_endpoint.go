package agent

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UiIndex serves files in the /ui/ prefix from a preconfigured directory
func (s *HTTPServer) UiIndex(resp http.ResponseWriter, req *http.Request) {
	// Invoke the handler
	start := time.Now()
	defer func() {
		s.logger.Printf("[DEBUG] http: Request %v (%v)", req.URL, time.Now().Sub(start))
	}()

	// Determine the file path
	file := strings.TrimPrefix(req.URL.Path, "/ui/")
	if file == "" {
		file = "index.html"
	}

	// Reject if it is relative
	if strings.Contains(file, "..") {
		s.logger.Printf("[WARN] http: Invalid file %s", file)
		resp.WriteHeader(404)
		return
	}
	path := filepath.Join(s.uiDir, file)

	// Attempt to open
	fh, err := os.Open(path)
	if err != nil {
		s.logger.Printf("[WARN] http: Failed to serve file %s: %v", path, err)
		resp.WriteHeader(404)
		return
	}
	defer fh.Close()

	// Get the file info
	info, err := fh.Stat()
	if err != nil {
		s.logger.Printf("[WARN] http: Failed to stat file %s: %v", path, err)
		resp.WriteHeader(404)
		return
	}

	// Serve the file
	http.ServeContent(resp, req, path, info.ModTime(), fh)
}
