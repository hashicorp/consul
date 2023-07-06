package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/agent/hcp/cloudcfg"
	"github.com/hashicorp/consul/lib"
)

func (s *HTTPHandlers) CloudLink(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var cloudCfg cloudcfg.CloudConfig
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&cloudCfg); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: err.Error()}
	}

	if s.agent.config.DevMode {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Cannot persist data in dev mode"}
	}

	if err := persistCloudConfig(s.agent.config.DataDir, &cloudCfg); err != nil {
		return nil, err
	}

	s.agent.Restart()
	return nil, nil
}

func persistCloudConfig(dataDir string, cloudCfg *cloudcfg.CloudConfig) error {
	// Hack - would have to retrieve this
	dir := filepath.Join(dataDir, "hcp-config")

	// Create subdir if it's not already there.
	if err := lib.EnsurePath(dir, true); err != nil {
		return fmt.Errorf("failed to ensure directory %q: %w", dir, err)
	}

	content, err := json.Marshal(cloudCfg)
	if err != nil {
		return err
	}

	file := filepath.Join(dir, "cloud.json")
	return os.WriteFile(file, content, 0600)
}
