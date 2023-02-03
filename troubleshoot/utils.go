package troubleshoot

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
