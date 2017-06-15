package structs

import (
	"testing"

	"github.com/hashicorp/consul/api"
)

func TestAgentStructs_HealthCheck(t *testing.T) {
	t.Parallel()
	def := CheckDefinition{}
	check := def.HealthCheck("node1")

	// Health checks default to critical state
	if check.Status != api.HealthCritical {
		t.Fatalf("bad: %v", check.Status)
	}
}
