package lib

import (
	"testing"
)

func TestUserAgent(t *testing.T) {
	projectURL = "https://consul-test.com"
	rt = "go5.0"
	versionFunc = func() string { return "1.2.3" }

	act := UserAgent()

	exp := "Consul/1.2.3 (+https://consul-test.com; go5.0)"
	if exp != act {
		t.Errorf("expected %q to be %q", act, exp)
	}
}
