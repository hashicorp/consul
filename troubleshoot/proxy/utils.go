package troubleshoot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

func (t *Troubleshoot) request(path string) ([]byte, error) {
	client := &http.Client{}
	url := fmt.Sprintf("http://%v:%s/%s", t.envoyAddr.IP, t.envoyAdminPort, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req.WithContext(context.Background()))
	if err != nil {
		return nil, err
	}

	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ErrBackendNotMounted")
	}

	rawConfig, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return rawConfig, nil
}

func (t *Troubleshoot) GetEnvoyConfigDump() error {
	configDumpRaw, err := t.request("config_dump")
	if err != nil {
		return err
	}

	config := &envoy_admin_v3.ConfigDump{}

	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = unmarshal.Unmarshal(configDumpRaw, config)
	if err != nil {
		return err
	}
	t.envoyConfigDump = config
	return nil
}

func envoyID(listenerName string) string {

	parts := strings.Split(listenerName, ":")
	if parts != nil || len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (t *Troubleshoot) parseClusters(clusters *envoy_admin_v3.Clusters) ([]string, error) {
	upstreams := []string{}

	for _, cs := range clusters.GetClusterStatuses() {
		if cs.Name == "local_app" || cs.Name == "local_agent" {
			continue
		}
		upstreams = append(upstreams, cs.GetName())
	}

	return upstreams, nil
}

func (t *Troubleshoot) getEnvoyClusters() error {
	clustersRaw, err := t.request("clusters?format=json")
	if err != nil {
		return err
	}
	clusters := &envoy_admin_v3.Clusters{}

	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = unmarshal.Unmarshal(clustersRaw, clusters)
	if err != nil {
		return err
	}

	t.envoyClusters = clusters
	return nil
}
