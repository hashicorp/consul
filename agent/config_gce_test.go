package agent

import (
	"log"
	"os"
	"testing"
)

func TestDiscoverGCEHosts(t *testing.T) {
	t.Parallel()
	if os.Getenv("GCE_PROJECT") == "" {
		t.Skip("GCE_PROJECT not set, skipping")
	}

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" && os.Getenv("GCE_CONFIG_CREDENTIALS") == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS or GCE_CONFIG_CREDENTIALS not set, skipping")
	}

	c := &Config{
		RetryJoinGCE: RetryJoinGCE{
			ProjectName:     os.Getenv("GCE_PROJECT"),
			ZonePattern:     os.Getenv("GCE_ZONE"),
			TagValue:        "consulrole-server",
			CredentialsFile: os.Getenv("GCE_CONFIG_CREDENTIALS"),
		},
	}

	servers, err := c.discoverGCEHosts(log.New(os.Stderr, "", log.LstdFlags))
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("bad: %v", servers)
	}
}
