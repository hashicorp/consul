package agent

import (
	"os"
	"testing"
)

func TestCatalogDatacenters(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	obj, err := srv.CatalogDatacenters(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	dcs := obj.([]string)
	if len(dcs) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}
