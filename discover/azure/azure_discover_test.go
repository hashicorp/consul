package azure

import (
	"log"
	"os"
	"testing"
)

func TestDiscover(t *testing.T) {
	subscriptionID := os.Getenv("ARM_SUBSCRIPTION_ID")
	tenantID := os.Getenv("ARM_TENANT_ID")
	clientID := os.Getenv("ARM_CLIENT_ID")
	clientSecret := os.Getenv("ARM_CLIENT_SECRET")
	environment := os.Getenv("ARM_ENVIRONMENT")

	if subscriptionID == "" || clientID == "" || clientSecret == "" || tenantID == "" {
		t.Skip("ARM_SUBSCRIPTION_ID, ARM_CLIENT_ID, ARM_CLIENT_SECRET and ARM_TENANT_ID " +
			"must be set to test Discover Azure Hosts")
	}

	if environment == "" {
		t.Log("Environments other than Public not supported at the moment")
	}

	cfg := map[string]string{
		"tenant_id":         tenantID,
		"client_id":         clientID,
		"subscription_id":   subscriptionID,
		"secret_access_key": clientSecret,
		"tag_name":          "type",
		"tag_value":         "Foundation",
	}

	l := log.New(os.Stderr, "", log.LstdFlags)
	addrs, err := Discover(cfg, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 3 {
		t.Fatalf("bad: %v", addrs)
	}
}
