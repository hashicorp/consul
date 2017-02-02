package kubernetes

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mholt/caddy"
	unversionedapi "k8s.io/client-go/1.5/pkg/api/unversioned"
)

func parseCidr(cidr string) net.IPNet {
	_, ipnet, _ := net.ParseCIDR(cidr)
	return *ipnet
}

func TestKubernetesParse(t *testing.T) {
	tests := []struct {
		description           string        // Human-facing description of test case
		input                 string        // Corefile data as string
		shouldErr             bool          // true if test case is exected to produce an error.
		expectedErrContent    string        // substring from the expected error. Empty for positive cases.
		expectedZoneCount     int           // expected count of defined zones.
		expectedNSCount       int           // expected count of namespaces.
		expectedResyncPeriod  time.Duration // expected resync period value
		expectedLabelSelector string        // expected label selector value
		expectedPodMode       string
		expectedCidrs         []net.IPNet
	}{
		// positive
		{
			"kubernetes keyword with one zone",
			`kubernetes coredns.local`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"kubernetes keyword with multiple zones",
			`kubernetes coredns.local test.local`,
			false,
			"",
			2,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"kubernetes keyword with zone and empty braces",
			`kubernetes coredns.local {
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"endpoint keyword with url",
			`kubernetes coredns.local {
	endpoint http://localhost:9090
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"namespaces keyword with one namespace",
			`kubernetes coredns.local {
	namespaces demo
}`,
			false,
			"",
			1,
			1,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"namespaces keyword with multiple namespaces",
			`kubernetes coredns.local {
	namespaces demo test
}`,
			false,
			"",
			1,
			2,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"resync period in seconds",
			`kubernetes coredns.local {
    resyncperiod 30s
}`,
			false,
			"",
			1,
			0,
			30 * time.Second,
			"",
			defaultPodMode,
			nil,
		},
		{
			"resync period in minutes",
			`kubernetes coredns.local {
    resyncperiod 15m
}`,
			false,
			"",
			1,
			0,
			15 * time.Minute,
			"",
			defaultPodMode,
			nil,
		},
		{
			"basic label selector",
			`kubernetes coredns.local {
    labels environment=prod
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"environment=prod",
			defaultPodMode,
			nil,
		},
		{
			"multi-label selector",
			`kubernetes coredns.local {
    labels environment in (production, staging, qa),application=nginx
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"application=nginx,environment in (production,qa,staging)",
			defaultPodMode,
			nil,
		},
		{
			"fully specified valid config",
			`kubernetes coredns.local test.local {
    resyncperiod 15m
	endpoint http://localhost:8080
	namespaces demo test
    labels environment in (production, staging, qa),application=nginx
}`,
			false,
			"",
			2,
			2,
			15 * time.Minute,
			"application=nginx,environment in (production,qa,staging)",
			defaultPodMode,
			nil,
		},
		// negative
		{
			"no kubernetes keyword",
			"",
			true,
			"Kubernetes setup called without keyword 'kubernetes' in Corefile",
			-1,
			-1,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"kubernetes keyword without a zone",
			`kubernetes`,
			true,
			"Zone name must be provided for kubernetes middleware",
			-1,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"endpoint keyword without an endpoint value",
			`kubernetes coredns.local {
    endpoint
}`,
			true,
			"Wrong argument count or unexpected line ending after 'endpoint'",
			-1,
			-1,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"namespace keyword without a namespace value",
			`kubernetes coredns.local {
	namespaces
}`,
			true,
			"Parse error: Wrong argument count or unexpected line ending after 'namespaces'",
			-1,
			-1,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
		{
			"resyncperiod keyword without a duration value",
			`kubernetes coredns.local {
    resyncperiod
}`,
			true,
			"Wrong argument count or unexpected line ending after 'resyncperiod'",
			-1,
			0,
			0 * time.Minute,
			"",
			defaultPodMode,
			nil,
		},
		{
			"resync period no units",
			`kubernetes coredns.local {
    resyncperiod 15
}`,
			true,
			"Unable to parse resync duration value. Value provided was ",
			-1,
			0,
			0 * time.Second,
			"",
			defaultPodMode,
			nil,
		},
		{
			"resync period invalid",
			`kubernetes coredns.local {
    resyncperiod abc
}`,
			true,
			"Unable to parse resync duration value. Value provided was ",
			-1,
			0,
			0 * time.Second,
			"",
			defaultPodMode,
			nil,
		},
		{
			"labels with no selector value",
			`kubernetes coredns.local {
    labels
}`,
			true,
			"Wrong argument count or unexpected line ending after 'labels'",
			-1,
			0,
			0 * time.Second,
			"",
			defaultPodMode,
			nil,
		},
		{
			"labels with invalid selector value",
			`kubernetes coredns.local {
    labels environment in (production, qa
}`,
			true,
			"Unable to parse label selector. Value provided was",
			-1,
			0,
			0 * time.Second,
			"",
			defaultPodMode,
			nil,
		},
		// pods disabled
		{
			"pods disabled",
			`kubernetes coredns.local {
	pods disabled
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			PodModeDisabled,
			nil,
		},
		// pods insecure
		{
			"pods insecure",
			`kubernetes coredns.local {
	pods insecure
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			PodModeInsecure,
			nil,
		},
		// pods verified
		{
			"pods verified",
			`kubernetes coredns.local {
	pods verified
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			PodModeVerified,
			nil,
		},
		// pods invalid
		{
			"invalid pods mode",
			`kubernetes coredns.local {
	pods giant_seed
}`,
			true,
			"Value for pods must be one of: disabled, verified, insecure",
			-1,
			0,
			defaultResyncPeriod,
			"",
			PodModeVerified,
			nil,
		},
		// cidrs ok
		{
			"valid cidrs",
			`kubernetes coredns.local {
	cidrs 10.0.0.0/24 10.0.1.0/24
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			[]net.IPNet{parseCidr("10.0.0.0/24"), parseCidr("10.0.1.0/24")},
		},
		// cidrs ok
		{
			"Invalid cidr: hard",
			`kubernetes coredns.local {
	cidrs hard dry
}`,
			true,
			"Invalid cidr: hard",
			-1,
			0,
			defaultResyncPeriod,
			"",
			defaultPodMode,
			nil,
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		k8sController, err := kubernetesParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error, but did not find error for input '%s'. Error was: '%v'", i, test.input, err)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
				continue
			}

			if test.shouldErr && (len(test.expectedErrContent) < 1) {
				t.Fatalf("Test %d: Test marked as expecting an error, but no expectedErrContent provided for input '%s'. Error was: '%v'", i, test.input, err)
			}

			if test.shouldErr && (test.expectedZoneCount >= 0) {
				t.Errorf("Test %d: Test marked as expecting an error, but provides value for expectedZoneCount!=-1 for input '%s'. Error was: '%v'", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
			continue
		}

		// No error was raised, so validate initialization of k8sController
		//     Zones
		foundZoneCount := len(k8sController.Zones)
		if foundZoneCount != test.expectedZoneCount {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with %d zones, instead found %d zones: '%v' for input '%s'", i, test.expectedZoneCount, foundZoneCount, k8sController.Zones, test.input)
		}

		//    Namespaces
		foundNSCount := len(k8sController.Namespaces)
		if foundNSCount != test.expectedNSCount {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with %d namespaces. Instead found %d namespaces: '%v' for input '%s'", i, test.expectedNSCount, foundNSCount, k8sController.Namespaces, test.input)
		}

		//    ResyncPeriod
		foundResyncPeriod := k8sController.ResyncPeriod
		if foundResyncPeriod != test.expectedResyncPeriod {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with resync period '%s'. Instead found period '%s' for input '%s'", i, test.expectedResyncPeriod, foundResyncPeriod, test.input)
		}

		//    Labels
		if k8sController.LabelSelector != nil {
			foundLabelSelectorString := unversionedapi.FormatLabelSelector(k8sController.LabelSelector)
			if foundLabelSelectorString != test.expectedLabelSelector {
				t.Errorf("Test %d: Expected kubernetes controller to be initialized with label selector '%s'. Instead found selector '%s' for input '%s'", i, test.expectedLabelSelector, foundLabelSelectorString, test.input)
			}
		}
		//    Pods
		foundPodMode := k8sController.PodMode
		if foundPodMode != test.expectedPodMode {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with pod mode '%s'. Instead found pod mode '%s' for input '%s'", i, test.expectedPodMode, foundPodMode, test.input)
		}

		//    Cidrs
		foundCidrs := k8sController.ReverseCidrs
		if len(foundCidrs) != len(test.expectedCidrs) {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with %d cidrs. Instead found %d cidrs for input '%s'", i, len(test.expectedCidrs), len(foundCidrs), test.input)
		}
		for j, cidr := range test.expectedCidrs {
			if cidr.String() != foundCidrs[j].String() {
				t.Errorf("Test %d: Expected kubernetes controller to be initialized with cidr '%s'. Instead found cidr '%s' for input '%s'", i, test.expectedCidrs[j].String(), foundCidrs[j].String(), test.input)
			}
		}

	}
}
