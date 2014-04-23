package agent

import (
	"net/http"
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

	file := strings.TrimPrefix(req.URL.Path, "/ui/")
	if file == "" {
		file = "index.html"
	}
	path := filepath.Join(s.uiDir, file)
	http.ServeFile(resp, req, path)
}
