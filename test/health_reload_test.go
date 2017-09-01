package test

import (
	"io/ioutil"
	"log"
	"net/http"
	"testing"
)

func TestHealthReload(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	// Corefile with for example without proxy section.
	corefile := `example.org:0 {
	health localhost:35080
}
`
	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	resp, err := http.Get("http://localhost:35080/health")
	if err != nil {
		t.Fatalf("Could not get health: %s", err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if x := string(body); x != "OK" {
		t.Fatalf("Expect OK, got %s", x)
	}
	resp.Body.Close()

	i, err = i.Restart(NewInput(corefile))
	if err != nil {
		t.Fatalf("Could not restart CoreDNS serving instance: %s", err)
	}

	defer i.Stop()

	resp, err = http.Get("http://localhost:35080/health")
	if err != nil {
		t.Fatalf("Could not get health: %s", err)
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Could not get resp.Body: %s", err)
	}
	if x := string(body); x != "OK" {
		t.Fatalf("Expect OK, got %s", x)
	}
	resp.Body.Close()
}
