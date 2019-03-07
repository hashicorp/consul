package health

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	h := newHealth(":0")

	if err := h.OnStartup(); err != nil {
		t.Fatalf("Unable to startup the health server: %v", err)
	}
	defer h.OnFinalShutdown()

	// Reconstruct the http address based on the port allocated by operating system.
	address := fmt.Sprintf("http://%s%s", h.ln.Addr().String(), path)

	response, err := http.Get(address)
	if err != nil {
		t.Fatalf("Unable to query %s: %v", address, err)
	}
	if response.StatusCode != 200 {
		t.Errorf("Invalid status code: expecting '200', got '%d'", response.StatusCode)
	}
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("Unable to get response body from %s: %v", address, err)
	}
	response.Body.Close()

	if string(content) != ok {
		t.Errorf("Invalid response body: expecting 'OK', got '%s'", string(content))
	}
}

func TestHealthLameduck(t *testing.T) {
	h := newHealth(":0")
	h.lameduck = 250 * time.Millisecond

	if err := h.OnStartup(); err != nil {
		t.Fatalf("Unable to startup the health server: %v", err)
	}

	h.OnFinalShutdown()
}
