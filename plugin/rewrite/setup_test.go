package rewrite

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestParse(t *testing.T) {
	c := caddy.NewTestController("dns", `rewrite`)
	_, err := rewriteParse(c)
	if err == nil {
		t.Errorf("Expected error but found nil for `rewrite`")
	}
	c = caddy.NewTestController("dns", `rewrite name`)
	_, err = rewriteParse(c)
	if err == nil {
		t.Errorf("Expected error but found nil for `rewrite name`")
	}
	c = caddy.NewTestController("dns", `rewrite name a.com b.com`)
	_, err = rewriteParse(c)
	if err != nil {
		t.Errorf("Expected success but found %s for `rewrite name a.com b.com`", err)
	}
}
