package file

import (
	"strings"
	"testing"
)

func TestParseNSEC3(t *testing.T) {
	_, err := Parse(strings.NewReader(nsec3_test), "miek.nl", "stdin")
	if err == nil {
		t.Fatalf("expected error when reading zone, got nothing")
	}
}

const nsec3_test = `miek.nl.		1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1460175181 14400 3600 604800 14400
miek.nl.		1800	IN	NS	omval.tednet.nl.
miek.nl.		1800	IN	NS	linode.atoom.net.
miek.nl.		1800	IN	NS	ext.ns.whyscream.net.
miek.nl.		1800	IN	NS	ns-ext.nlnetlabs.nl.
miek.nl.		0	IN	NSEC3PARAM 1 0 5 A3DEBC9CC4F695C7`
