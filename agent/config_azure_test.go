package agent

import (
	"log"
	"os"
	"testing"
)

func TestDiscoverAzureHosts(t *testing.T) {
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

	c := &Config{
		RetryJoinAzure: RetryJoinAzure{
			SubscriptionID:  subscriptionID,
			ClientID:        clientID,
			SecretAccessKey: clientSecret,
			TenantID:        tenantID,
			TagName:         "type",
			TagValue:        "Foundation",
		},
	}

	servers, err := c.discoverAzureHosts(log.New(os.Stderr, "", log.LstdFlags))
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("bad: %v", servers)
	}
}
