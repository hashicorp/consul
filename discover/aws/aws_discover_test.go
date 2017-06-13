package aws

import (
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

	c := &Config{
		Region:   os.Getenv("AWS_REGION"),
		TagKey:   "ConsulRole",
		TagValue: "Server",
	}

	l := log.New(os.Stderr, "", log.LstdFlags)
	addrs, err := Discover(c, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 3 {
		t.Fatalf("bad: %v", addrs)
	}
}
