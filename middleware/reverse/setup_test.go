package reverse

import (
	"net"
	"reflect"
	"regexp"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupParse(t *testing.T) {

	_, net4, _ := net.ParseCIDR("10.1.1.0/24")
	_, net6, _ := net.ParseCIDR("fd01::/64")

	regexIP4wildcard, _ := regexp.Compile("^.*ip-" + regexMatchV4 + "\\.domain\\.com\\.$")
	regexIP6, _ := regexp.Compile("^ip-" + regexMatchV6 + "\\.domain\\.com\\.$")
	regexIpv4dynamic, _ := regexp.Compile("^dynamic-" + regexMatchV4 + "-intern\\.dynamic\\.domain\\.com\\.$")
	regexIpv6dynamic, _ := regexp.Compile("^dynamic-" + regexMatchV6 + "-intern\\.dynamic\\.domain\\.com\\.$")
	regexIpv4vpndynamic, _ := regexp.Compile("^dynamic-" + regexMatchV4 + "-vpn\\.dynamic\\.domain\\.com\\.$")

	serverBlockKeys := []string{"domain.com.:8053", "dynamic.domain.com.:8053"}

	tests := []struct {
		inputFileRules string
		shouldErr      bool
		networks       networks
	}{
		{
			// with defaults
			`reverse fd01::/64`,
			false,
			networks{network{
				IPnet:        net6,
				Template:     "ip-{ip}.domain.com.",
				Zone:         "domain.com.",
				TTL:          60,
				RegexMatchIP: regexIP6,
			}},
		},
		{
			`reverse`,
			true,
			networks{},
		},
		{
			//no cidr
			`reverse 10.1.1.1`,
			true,
			networks{},
		},
		{
			//no cidr
			`reverse 10.1.1.0/16 fd00::`,
			true,
			networks{},
		},
		{
			// invalid key
			`reverse 10.1.1.0/24 {
				notavailable
			}`,
			true,
			networks{},
		},
		{
			// no domain suffix
			`reverse 10.1.1.0/24 {
				hostname ip-{ip}.
			}`,
			true,
			networks{},
		},
		{
			// hostname requires an second arg
			`reverse 10.1.1.0/24 {
				hostname
			}`,
			true,
			networks{},
		},
		{
			// template breaks regex compile
			`reverse 10.1.1.0/24 {
				hostname ip-{[-x
			}`,
			true,
			networks{},
		},
		{
			// ttl requires an (u)int
			`reverse 10.1.1.0/24 {
				ttl string
			}`,
			true,
			networks{},
		},
		{
			`reverse fd01::/64 {
				hostname dynamic-{ip}-intern.{zone[2]}
				ttl 50
			}
			reverse 10.1.1.0/24 {
				hostname dynamic-{ip}-vpn.{zone[2]}
				fallthrough
			}`,
			false,
			networks{network{
				IPnet:        net6,
				Template:     "dynamic-{ip}-intern.dynamic.domain.com.",
				Zone:         "dynamic.domain.com.",
				TTL:          50,
				RegexMatchIP: regexIpv6dynamic,
			}, network{
				IPnet:        net4,
				Template:     "dynamic-{ip}-vpn.dynamic.domain.com.",
				Zone:         "dynamic.domain.com.",
				TTL:          60,
				RegexMatchIP: regexIpv4vpndynamic,
			}},
		},
		{
			// multiple networks in one stanza
			`reverse fd01::/64 10.1.1.0/24 {
				hostname dynamic-{ip}-intern.{zone[2]}
				ttl 50
				fallthrough
			}`,
			false,
			networks{network{
				IPnet:        net6,
				Template:     "dynamic-{ip}-intern.dynamic.domain.com.",
				Zone:         "dynamic.domain.com.",
				TTL:          50,
				RegexMatchIP: regexIpv6dynamic,
			}, network{
				IPnet:        net4,
				Template:     "dynamic-{ip}-intern.dynamic.domain.com.",
				Zone:         "dynamic.domain.com.",
				TTL:          50,
				RegexMatchIP: regexIpv4dynamic,
			}},
		},
		{
			// fix domain in template
			`reverse fd01::/64 {
				hostname dynamic-{ip}-intern.dynamic.domain.com
				ttl 300
				fallthrough
			}`,
			false,
			networks{network{
				IPnet:        net6,
				Template:     "dynamic-{ip}-intern.dynamic.domain.com.",
				Zone:         "dynamic.domain.com.",
				TTL:          300,
				RegexMatchIP: regexIpv6dynamic,
			}},
		},
		{
			`reverse 10.1.1.0/24 {
				hostname ip-{ip}.{zone[1]}
				ttl 50
			    wildcard
				fallthrough
			}`,
			false,
			networks{network{
				IPnet:        net4,
				Template:     "ip-{ip}.domain.com.",
				Zone:         "domain.com.",
				TTL:          50,
				RegexMatchIP: regexIP4wildcard,
			}},
		},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		c.ServerBlockKeys = serverBlockKeys
		networks, _, err := reverseParse(c)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error", i)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		}
		for j, n := range networks {
			reflect.DeepEqual(test.networks[j], n)
			if !reflect.DeepEqual(test.networks[j], n) {
				t.Fatalf("Test %d/%d expected %v, got %v", i, j, test.networks[j], n)
			}
		}
	}
}
