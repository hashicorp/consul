package agent

import (
	"reflect"
	"testing"

	discover "github.com/hashicorp/go-discover"
)

// if this test fails check the _ imports of go-discover/provider/* packages
// in retry_join.go
func TestGoDiscoverRegistration(t *testing.T) {
	got := discover.ProviderNames()
	want := []string{"aws", "azure", "gce"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got go-discover providers %v want %v", got, want)
	}
}
