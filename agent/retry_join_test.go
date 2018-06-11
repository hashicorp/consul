package agent

import (
	"reflect"
	"testing"

	discover "github.com/hashicorp/go-discover"
)

func TestGoDiscoverRegistration(t *testing.T) {
	d, err := discover.New()
	if err != nil {
		t.Fatal(err)
	}
	got := d.Names()
	want := []string{"aliyun", "aws", "azure", "digitalocean", "gce", "os", "scaleway", "softlayer", "triton"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got go-discover providers %v want %v", got, want)
	}
}
