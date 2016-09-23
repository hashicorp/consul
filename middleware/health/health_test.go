package health

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	// We use a random port instead of a fixed port like 8080 that may have been
	// occupied by some other process.
	h := health{Addr: ":0"}
	if err := h.Startup(); err != nil {
		t.Fatalf("Unable to startup the health server: %v", err)
	}
	defer h.Shutdown()

	// Reconstruct the http address based on the port allocated by operating system.
	address := fmt.Sprintf("http://%s%s", h.ln.Addr().String(), path)

	response, err := http.Get(address)
	if err != nil {
		t.Fatalf("Unable to query %s: %v", address, err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Errorf("Invalid status code: expecting '200', got '%d'", response.StatusCode)
	}
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("Unable to get response body from %s: %v", address, err)
	}
	if string(content) != "OK" {
		t.Errorf("Invalid response body: expecting 'OK', got '%s'", string(content))
	}
}
