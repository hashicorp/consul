package reverse

import (
	"net"
	"reflect"
	"regexp"
	"testing"
)

// Test converting from hostname to IP and back again to hostname
func TestNetworkConversion(t *testing.T) {

	_, net4, _ := net.ParseCIDR("10.1.1.0/24")
	_, net6, _ := net.ParseCIDR("fd01::/64")

	regexIP4, _ := regexp.Compile("^dns-" + regexMatchV4 + "\\.domain\\.internal\\.$")
	regexIP6, _ := regexp.Compile("^dns-" + regexMatchV6 + "\\.domain\\.internal\\.$")

	tests := []struct {
		network    network
		resultHost string
		resultIP   net.IP
	}{
		{
			network{
				IPnet:        net4,
				Template:     "dns-{ip}.domain.internal.",
				RegexMatchIP: regexIP4,
			},
			"dns-10.1.1.23.domain.internal.",
			net.ParseIP("10.1.1.23"),
		},
		{
			network{
				IPnet:        net6,
				Template:     "dns-{ip}.domain.internal.",
				RegexMatchIP: regexIP6,
			},
			"dns-fd01000000000000000000000000a32f.domain.internal.",
			net.ParseIP("fd01::a32f"),
		},
	}

	for i, test := range tests {
		resultIP := test.network.hostnameToIP(test.resultHost)
		if !reflect.DeepEqual(test.resultIP, resultIP) {
			t.Fatalf("Test %d expected %v, got %v", i, test.resultIP, resultIP)
		}

		resultHost := test.network.ipToHostname(test.resultIP)
		if !reflect.DeepEqual(test.resultHost, resultHost) {
			t.Fatalf("Test %d expected %v, got %v", i, test.resultHost, resultHost)
		}
	}
}

func TestNetworkHostnameToIP(t *testing.T) {

	_, net4, _ := net.ParseCIDR("10.1.1.0/24")
	_, net6, _ := net.ParseCIDR("fd01::/64")

	regexIP4, _ := regexp.Compile("^dns-" + regexMatchV4 + "\\.domain\\.internal\\.$")
	regexIP6, _ := regexp.Compile("^dns-" + regexMatchV6 + "\\.domain\\.internal\\.$")

	// Test regex does NOT match
	// All this test should return nil
	testsNil := []struct {
		network  network
		hostname string
	}{
		{
			network{
				IPnet:        net4,
				RegexMatchIP: regexIP4,
			},
			// domain does not match
			"dns-10.1.1.23.domain.internals.",
		},
		{
			network{
				IPnet:        net4,
				RegexMatchIP: regexIP4,
			},
			// IP does match / contain in subnet
			"dns-200.1.1.23.domain.internals.",
		},
		{
			network{
				IPnet:        net4,
				RegexMatchIP: regexIP4,
			},
			// template does not match
			"dns-10.1.1.23-x.domain.internal.",
		},
		{
			network{
				IPnet:        net4,
				RegexMatchIP: regexIP4,
			},
			// template does not match
			"IP-dns-10.1.1.23.domain.internal.",
		},
		{
			network{
				IPnet:        net6,
				RegexMatchIP: regexIP6,
			},
			// template does not match
			"dnx-fd01000000000000000000000000a32f.domain.internal.",
		},
		{
			network{
				IPnet:        net6,
				RegexMatchIP: regexIP6,
			},
			// no valid v6 (missing one 0, only 31 chars)
			"dns-fd0100000000000000000000000a32f.domain.internal.",
		},
		{
			network{
				IPnet:        net6,
				RegexMatchIP: regexIP6,
			},
			// IP does match / contain in subnet
			"dns-ed01000000000000000000000000a32f.domain.internal.",
		},
	}

	for i, test := range testsNil {
		resultIP := test.network.hostnameToIP(test.hostname)
		if resultIP != nil {
			t.Fatalf("Test %d expected nil, got %v", i, resultIP)
		}
	}
}
