package kubernetes

import (
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/fall"

	"github.com/coredns/coredns/plugin/proxy"
	"github.com/mholt/caddy"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubernetesParse(t *testing.T) {
	tests := []struct {
		input                 string        // Corefile data as string
		shouldErr             bool          // true if test case is expected to produce an error.
		expectedErrContent    string        // substring from the expected error. Empty for positive cases.
		expectedZoneCount     int           // expected count of defined zones.
		expectedNSCount       int           // expected count of namespaces.
		expectedResyncPeriod  time.Duration // expected resync period value
		expectedLabelSelector string        // expected label selector value
		expectedPodMode       string
		expectedFallthrough   fall.F
		expectedUpstreams     []string
	}{
		// positive
		{
			`kubernetes coredns.local`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local test.local`,
			false,
			"",
			2,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	endpoint http://localhost:9090 http://localhost:9091
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	namespaces demo
}`,
			false,
			"",
			1,
			1,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	namespaces demo test
}`,
			false,
			"",
			1,
			2,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    resyncperiod 30s
}`,
			false,
			"",
			1,
			0,
			30 * time.Second,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    resyncperiod 15m
}`,
			false,
			"",
			1,
			0,
			15 * time.Minute,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    labels environment=prod
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"environment=prod",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    labels environment in (production, staging, qa),application=nginx
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"application=nginx,environment in (production,qa,staging)",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local test.local {
    resyncperiod 15m
	endpoint http://localhost:8080
	namespaces demo test
    labels environment in (production, staging, qa),application=nginx
    fallthrough
}`,
			false,
			"",
			2,
			2,
			15 * time.Minute,
			"application=nginx,environment in (production,qa,staging)",
			podModeDisabled,
			fall.Root,
			nil,
		},
		// negative
		{
			`kubernetes coredns.local {
    endpoint
}`,
			true,
			"rong argument count or unexpected line ending",
			-1,
			-1,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	namespaces
}`,
			true,
			"rong argument count or unexpected line ending",
			-1,
			-1,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    resyncperiod
}`,
			true,
			"rong argument count or unexpected line ending",
			-1,
			0,
			0 * time.Minute,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    resyncperiod 15
}`,
			true,
			"unable to parse resync duration value",
			-1,
			0,
			0 * time.Second,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    resyncperiod abc
}`,
			true,
			"unable to parse resync duration value",
			-1,
			0,
			0 * time.Second,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    labels
}`,
			true,
			"rong argument count or unexpected line ending",
			-1,
			0,
			0 * time.Second,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
    labels environment in (production, qa
}`,
			true,
			"unable to parse label selector",
			-1,
			0,
			0 * time.Second,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		// pods disabled
		{
			`kubernetes coredns.local {
	pods disabled
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		// pods insecure
		{
			`kubernetes coredns.local {
	pods insecure
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeInsecure,
			fall.Zero,
			nil,
		},
		// pods verified
		{
			`kubernetes coredns.local {
	pods verified
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeVerified,
			fall.Zero,
			nil,
		},
		// pods invalid
		{
			`kubernetes coredns.local {
	pods giant_seed
}`,
			true,
			"rong value for pods",
			-1,
			0,
			defaultResyncPeriod,
			"",
			podModeVerified,
			fall.Zero,
			nil,
		},
		// fallthrough with zones
		{
			`kubernetes coredns.local {
	fallthrough ip6.arpa inaddr.arpa foo.com
}`,
			false,
			"rong argument count",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.F{Zones: []string{"ip6.arpa.", "inaddr.arpa.", "foo.com."}},
			nil,
		},
		// Valid upstream
		{
			`kubernetes coredns.local {
	upstream 13.14.15.16:53
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			[]string{"13.14.15.16:53"},
		},
		// Invalid upstream
		{
			`kubernetes coredns.local {
	upstream 13.14.15.16orange
}`,
			true,
			"not an IP address or file: \"13.14.15.16orange\"",
			-1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		// More than one Kubernetes not allowed
		{
			`kubernetes coredns.local
kubernetes cluster.local`,
			true,
			"this plugin",
			-1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	kubeconfig
}`,
			true,
			"Wrong argument count or unexpected line ending after",
			-1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	kubeconfig file context extraarg
}`,
			true,
			"Wrong argument count or unexpected line ending after",
			-1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
			nil,
		},
		{
			`kubernetes coredns.local {
	kubeconfig file context
}`,
			false,
			"",
			1,
			0,
			defaultResyncPeriod,
			"",
			podModeDisabled,
			fall.Zero,
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
		foundResyncPeriod := k8sController.opts.resyncPeriod
		if foundResyncPeriod != test.expectedResyncPeriod {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with resync period '%s'. Instead found period '%s' for input '%s'", i, test.expectedResyncPeriod, foundResyncPeriod, test.input)
		}

		//    Labels
		if k8sController.opts.labelSelector != nil {
			foundLabelSelectorString := meta.FormatLabelSelector(k8sController.opts.labelSelector)
			if foundLabelSelectorString != test.expectedLabelSelector {
				t.Errorf("Test %d: Expected kubernetes controller to be initialized with label selector '%s'. Instead found selector '%s' for input '%s'", i, test.expectedLabelSelector, foundLabelSelectorString, test.input)
			}
		}
		//    Pods
		foundPodMode := k8sController.podMode
		if foundPodMode != test.expectedPodMode {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with pod mode '%s'. Instead found pod mode '%s' for input '%s'", i, test.expectedPodMode, foundPodMode, test.input)
		}

		// fallthrough
		if !k8sController.Fall.Equal(test.expectedFallthrough) {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with fallthrough '%v'. Instead found fallthrough '%v' for input '%s'", i, test.expectedFallthrough, k8sController.Fall, test.input)
		}
		// upstream
		var foundUpstreams *[]proxy.Upstream
		if k8sController.Upstream.Forward != nil {
			foundUpstreams = k8sController.Upstream.Forward.Upstreams
		}
		if test.expectedUpstreams == nil {
			if foundUpstreams != nil {
				t.Errorf("Test %d: Expected kubernetes controller to not be initialized with upstreams for input '%s'", i, test.input)
			}
		} else {
			if foundUpstreams == nil {
				t.Errorf("Test %d: Expected kubernetes controller to be initialized with upstreams for input '%s'", i, test.input)
			} else {
				if len(*foundUpstreams) != len(test.expectedUpstreams) {
					t.Errorf("Test %d: Expected kubernetes controller to be initialized with %d upstreams. Instead found %d upstreams for input '%s'", i, len(test.expectedUpstreams), len(*foundUpstreams), test.input)
				}
				for j, want := range test.expectedUpstreams {
					got := (*foundUpstreams)[j].Select().Name
					if got != want {
						t.Errorf("Test %d: Expected kubernetes controller to be initialized with upstream '%s'. Instead found upstream '%s' for input '%s'", i, want, got, test.input)
					}
				}

			}
		}
	}
}

func TestKubernetesParseEndpointPodNames(t *testing.T) {
	tests := []struct {
		input                string // Corefile data as string
		shouldErr            bool   // true if test case is expected to produce an error.
		expectedErrContent   string // substring from the expected error. Empty for positive cases.
		expectedEndpointMode bool
	}{
		// valid endpoints mode
		{
			`kubernetes coredns.local {
	endpoint_pod_names
}`,
			false,
			"",
			true,
		},
		// endpoints invalid
		{
			`kubernetes coredns.local {
	endpoint_pod_names giant_seed
}`,
			true,
			"rong argument count or unexpected",
			false,
		},
		// endpoint not set
		{
			`kubernetes coredns.local {
}`,
			false,
			"",
			false,
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

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
			continue
		}

		// Endpoints
		foundEndpointNameMode := k8sController.endpointNameMode
		if foundEndpointNameMode != test.expectedEndpointMode {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with endpoints mode '%v'. Instead found endpoints mode '%v' for input '%s'", i, test.expectedEndpointMode, foundEndpointNameMode, test.input)
		}
	}
}

func TestKubernetesParseNoEndpoints(t *testing.T) {
	tests := []struct {
		input                 string // Corefile data as string
		shouldErr             bool   // true if test case is expected to produce an error.
		expectedErrContent    string // substring from the expected error. Empty for positive cases.
		expectedEndpointsInit bool
	}{
		// valid
		{
			`kubernetes coredns.local {
	noendpoints
}`,
			false,
			"",
			false,
		},
		// invalid
		{
			`kubernetes coredns.local {
	noendpoints ixnay on the endpointsay
}`,
			true,
			"rong argument count or unexpected",
			true,
		},
		// not set
		{
			`kubernetes coredns.local {
}`,
			false,
			"",
			true,
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

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
			continue
		}

		foundEndpointsInit := k8sController.opts.initEndpointsCache
		if foundEndpointsInit != test.expectedEndpointsInit {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with endpoints watch '%v'. Instead found endpoints watch '%v' for input '%s'", i, test.expectedEndpointsInit, foundEndpointsInit, test.input)
		}
	}
}

func TestKubernetesParseIgnoreEmptyService(t *testing.T) {
	tests := []struct {
		input                 string // Corefile data as string
		shouldErr             bool   // true if test case is expected to produce an error.
		expectedErrContent    string // substring from the expected error. Empty for positive cases.
		expectedEndpointsInit bool
	}{
		// valid
		{
			`kubernetes coredns.local {
	ignore empty_service
}`,
			false,
			"",
			true,
		},
		// invalid
		{
			`kubernetes coredns.local {
	ignore ixnay on the endpointsay
}`,
			true,
			"unable to parse ignore value",
			false,
		},
		{
			`kubernetes coredns.local {
	ignore empty_service ixnay on the endpointsay
}`,
			false,
			"",
			true,
		},
		// not set
		{
			`kubernetes coredns.local {
}`,
			false,
			"",
			false,
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

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
			continue
		}

		foundIgnoreEmptyService := k8sController.opts.ignoreEmptyService
		if foundIgnoreEmptyService != test.expectedEndpointsInit {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with ignore empty_service '%v'. Instead found ignore empty_service watch '%v' for input '%s'", i, test.expectedEndpointsInit, foundIgnoreEmptyService, test.input)
		}
	}
}
