package gce

import (
	"fmt"
	"log"
	"os"
	"testing"
)

func TestDiscover(t *testing.T) {
	t.Parallel()
	if os.Getenv("GCE_PROJECT") == "" {
		t.Skip("GCE_PROJECT not set, skipping")
	}

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" && os.Getenv("GCE_CONFIG_CREDENTIALS") == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS or GCE_CONFIG_CREDENTIALS not set, skipping")
	}

	cfg := fmt.Sprintf("project_name=%s zone_pattern=%s tag_value=%s credentials_file=%s",
		os.Getenv("GCE_PROJECT"), os.Getenv("GCE_ZONE"), "consul-server", os.Getenv("GCE_CONFIG_CREDENTIALS"))

	l := log.New(os.Stderr, "", log.LstdFlags)
	addrs, err := Discover(cfg, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 3 {
		t.Fatalf("bad: %v", addrs)
	}
}
