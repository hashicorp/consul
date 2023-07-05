package agent

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/lib"
)

func (s *HTTPHandlers) CloudLink(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var clientID string
	if clientID = req.URL.Query().Get("client_id"); clientID == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing client ID"}
	}

	var clientSecret string
	if clientSecret = req.URL.Query().Get("client_secret"); clientSecret == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing client secret"}
	}

	var resourceID string
	if resourceID = req.URL.Query().Get("resource_id"); resourceID == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing resource ID"}
	}

	if err := persistCloudConfig(clientID, clientSecret, resourceID); err != nil {
		return nil, err
	}

	// TODO: restart

	return true, nil
}

func persistCloudConfig(clientID, clientSecret, resourceID string) error {
	// Hack - would have to retrieve this
	dir := "opt/consul/data/hcp-config"

	if err := lib.EnsurePath(dir, true); err != nil {
		// Create subdir if it's not already there.
		return fmt.Errorf("failed to ensure directory %q: %w", dir, err)
	}

	content := fmt.Sprintf(`
{
	"cloud": {
		"resource_id": "%s",
		"client_id": "%s",
		"client_secret": "%s"
	}
}
`, resourceID, clientID, clientSecret)

	file := filepath.Join(dir, "cloud.json")
	return os.WriteFile(file, []byte(content), 0600)
}
