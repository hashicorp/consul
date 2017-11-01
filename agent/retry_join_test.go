package agent

import (
	"reflect"
	"testing"

	discover "github.com/hashicorp/go-discover"
)

func TestGoDiscoverRegistration(t *testing.T) {
	d := discover.Discover{}
	got := d.Names()
	want := []string{"aliyun", "aws", "azure", "digitalocean", "gce", "os", "scaleway", "softlayer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got go-discover providers %v want %v", got, want)
	}
}
