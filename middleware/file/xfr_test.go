package file

import (
	"fmt"
	"strings"
)

func ExampleZone_All() {
	zone, err := Parse(strings.NewReader(dbMiekNL), testzone, "stdin")
	if err != nil {
		return
	}
	records := zone.All()
	for _, r := range records {
		fmt.Printf("%+v\n", r)
	}
	// Output
	// xfr_test.go:15: miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400
	// xfr_test.go:15: www.miek.nl.	1800	IN	CNAME	a.miek.nl.
	// xfr_test.go:15: miek.nl.	1800	IN	NS	linode.atoom.net.
	// xfr_test.go:15: miek.nl.	1800	IN	NS	ns-ext.nlnetlabs.nl.
	// xfr_test.go:15: miek.nl.	1800	IN	NS	omval.tednet.nl.
	// xfr_test.go:15: miek.nl.	1800	IN	NS	ext.ns.whyscream.net.
	// xfr_test.go:15: miek.nl.	1800	IN	MX	1 aspmx.l.google.com.
	// xfr_test.go:15: miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com.
	// xfr_test.go:15: miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com.
	// xfr_test.go:15: miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com.
	// xfr_test.go:15: miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com.
	// xfr_test.go:15: miek.nl.	1800	IN	A	139.162.196.78
	// xfr_test.go:15: miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735
	// xfr_test.go:15: archive.miek.nl.	1800	IN	CNAME	a.miek.nl.
	// xfr_test.go:15: a.miek.nl.	1800	IN	A	139.162.196.78
	// xfr_test.go:15: a.miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735
}
