package agent

import (
	"log"
	"os"
	"testing"
)

func TestDiscoverEC2Hosts(t *testing.T) {
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
		RetryJoinEC2: RetryJoinEC2{
			Region:   os.Getenv("AWS_REGION"),
			TagKey:   "ConsulRole",
			TagValue: "Server",
		},
	}

	servers, err := c.discoverEc2Hosts(&log.Logger{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("bad: %v", servers)
	}
}
