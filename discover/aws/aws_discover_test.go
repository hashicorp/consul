package aws

import (
	"fmt"
	"log"
	"os"
	"testing"
)

func TestDiscover(t *testing.T) {
	t.Parallel()
	if os.Getenv("AWS_REGION") == "" {
		t.Skip("AWS_REGION not set, skipping")
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS_ACCESS_KEY_ID not set, skipping")
	}

	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		t.Skip("AWS_SECRET_ACCESS_KEY not set, skipping")
	}

	cfg := fmt.Sprintf("region=%s tag_key=%s tag_value=%s access_key_id=%s secret_access_key=%s",
		os.Getenv("AWS_REGION"), "consul-role", "server", os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))

	l := log.New(os.Stderr, "", log.LstdFlags)
	addrs, err := Discover(cfg, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 3 {
		t.Fatalf("bad: %v", addrs)
	}
}
