package rewrite

import (
	"strings"
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

	c = caddy.NewTestController("dns",
		`rewrite stop {
    name regex foo bar
    answer name bar foo
}`)
	_, err = rewriteParse(c)
	if err != nil {
		t.Errorf("Expected success but found %s for valid response rewrite", err)
	}

	c = caddy.NewTestController("dns", `rewrite stop name regex foo bar answer name bar foo`)
	_, err = rewriteParse(c)
	if err != nil {
		t.Errorf("Expected success but found %s for valid response rewrite", err)
	}

	c = caddy.NewTestController("dns",
		`rewrite stop {
    name regex foo bar
    answer name bar foo
    name baz qux
}`)
	_, err = rewriteParse(c)
	if err == nil {
		t.Errorf("Expected error but got success for invalid response rewrite")
	} else if !strings.Contains(err.Error(), "must consist only of") {
		t.Errorf("Got wrong error for invalid response rewrite: %v", err.Error())
	}

	c = caddy.NewTestController("dns",
		`rewrite stop {
    answer name bar foo
    name regex foo bar
}`)
	_, err = rewriteParse(c)
	if err == nil {
		t.Errorf("Expected error but got success for invalid response rewrite")
	} else if !strings.Contains(err.Error(), "must begin with a name rule") {
		t.Errorf("Got wrong error for invalid response rewrite: %v", err.Error())
	}
}
