package kubernetes

import (
	"strings"
	"testing"
	"time"

	"github.com/mholt/caddy"
	unversionedapi "k8s.io/client-go/1.5/pkg/api/unversioned"
)

func TestKubernetesParse(t *testing.T) {
	tests := []struct {
		description           string        // Human-facing description of test case
		input                 string        // Corefile data as string
		shouldErr             bool          // true if test case is exected to produce an error.
		expectedErrContent    string        // substring from the expected error. Empty for positive cases.
		expectedZoneCount     int           // expected count of defined zones.
		expectedNTValid       bool          // NameTemplate to be initialized and valid
		expectedNSCount       int           // expected count of namespaces.
		expectedResyncPeriod  time.Duration // expected resync period value
		expectedLabelSelector string        // expected label selector value
	}{
		// positive
		{
			"kubernetes keyword with one zone",
			`kubernetes coredns.local`,
			false,
			"",
			1,
			true,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"kubernetes keyword with multiple zones",
			`kubernetes coredns.local test.local`,
			false,
			"",
			2,
			true,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"kubernetes keyword with zone and empty braces",
			`kubernetes coredns.local {
}`,
			false,
			"",
			1,
			true,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"endpoint keyword with url",
			`kubernetes coredns.local {
	endpoint http://localhost:9090
}`,
			false,
			"",
			1,
			true,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"template keyword with valid template",
			`kubernetes coredns.local {
	template {service}.{namespace}.{zone}
}`,
			false,
			"",
			1,
			true,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"namespaces keyword with one namespace",
			`kubernetes coredns.local {
	namespaces demo
}`,
			false,
			"",
			1,
			true,
			1,
			defaultResyncPeriod,
			"",
		},
		{
			"namespaces keyword with multiple namespaces",
			`kubernetes coredns.local {
	namespaces demo test
}`,
			false,
			"",
			1,
			true,
			2,
			defaultResyncPeriod,
			"",
		},
		{
			"resync period in seconds",
			`kubernetes coredns.local {
    resyncperiod 30s
}`,
			false,
			"",
			1,
			true,
			0,
			30 * time.Second,
			"",
		},
		{
			"resync period in minutes",
			`kubernetes coredns.local {
    resyncperiod 15m
}`,
			false,
			"",
			1,
			true,
			0,
			15 * time.Minute,
			"",
		},
		{
			"basic label selector",
			`kubernetes coredns.local {
    labels environment=prod
}`,
			false,
			"",
			1,
			true,
			0,
			defaultResyncPeriod,
			"environment=prod",
		},
		{
			"multi-label selector",
			`kubernetes coredns.local {
    labels environment in (production, staging, qa),application=nginx
}`,
			false,
			"",
			1,
			true,
			0,
			defaultResyncPeriod,
			"application=nginx,environment in (production,qa,staging)",
		},
		{
			"fully specified valid config",
			`kubernetes coredns.local test.local {
    resyncperiod 15m
	endpoint http://localhost:8080
	template {service}.{namespace}.{zone}
	namespaces demo test
    labels environment in (production, staging, qa),application=nginx
}`,
			false,
			"",
			2,
			true,
			2,
			15 * time.Minute,
			"application=nginx,environment in (production,qa,staging)",
		},
		// negative
		{
			"no kubernetes keyword",
			"",
			true,
			"Kubernetes setup called without keyword 'kubernetes' in Corefile",
			-1,
			false,
			-1,
			defaultResyncPeriod,
			"",
		},
		{
			"kubernetes keyword without a zone",
			`kubernetes`,
			true,
			"Zone name must be provided for kubernetes middleware",
			-1,
			true,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"endpoint keyword without an endpoint value",
			`kubernetes coredns.local {
    endpoint
}`,
			true,
			"Wrong argument count or unexpected line ending after 'endpoint'",
			-1,
			true,
			-1,
			defaultResyncPeriod,
			"",
		},
		{
			"template keyword without a template value",
			`kubernetes coredns.local {
    template
}`,
			true,
			"Wrong argument count or unexpected line ending after 'template'",
			-1,
			false,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"template keyword with an invalid template value",
			`kubernetes coredns.local {
    template {namespace}.{zone}
}`,
			true,
			"Record name template does not pass NameTemplate validation",
			-1,
			false,
			0,
			defaultResyncPeriod,
			"",
		},
		{
			"namespace keyword without a namespace value",
			`kubernetes coredns.local {
	namespaces
}`,
			true,
			"Parse error: Wrong argument count or unexpected line ending after 'namespaces'",
			-1,
			true,
			-1,
			defaultResyncPeriod,
			"",
		},
		{
			"resyncperiod keyword without a duration value",
			`kubernetes coredns.local {
    resyncperiod
}`,
			true,
			"Wrong argument count or unexpected line ending after 'resyncperiod'",
			-1,
			true,
			0,
			0 * time.Minute,
			"",
		},
		{
			"resync period no units",
			`kubernetes coredns.local {
    resyncperiod 15
}`,
			true,
			"Unable to parse resync duration value. Value provided was ",
			-1,
			true,
			0,
			0 * time.Second,
			"",
		},
		{
			"resync period invalid",
			`kubernetes coredns.local {
    resyncperiod abc
}`,
			true,
			"Unable to parse resync duration value. Value provided was ",
			-1,
			true,
			0,
			0 * time.Second,
			"",
		},
		{
			"labels with no selector value",
			`kubernetes coredns.local {
    labels
}`,
			true,
			"Wrong argument count or unexpected line ending after 'labels'",
			-1,
			true,
			0,
			0 * time.Second,
			"",
		},
		{
			"labels with invalid selector value",
			`kubernetes coredns.local {
    labels environment in (production, qa
}`,
			true,
			"Unable to parse label selector. Value provided was",
			-1,
			true,
			0,
			0 * time.Second,
			"",
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

		//    NameTemplate
		if k8sController.NameTemplate == nil {
			t.Errorf("Test %d: Expected kubernetes controller to be initialized with a NameTemplate. Instead found '%v' for input '%s'", i, k8sController.NameTemplate, test.input)
		} else {
			foundNTValid := k8sController.NameTemplate.IsValid()
			if foundNTValid != test.expectedNTValid {
				t.Errorf("Test %d: Expected NameTemplate validity to be '%v', instead found '%v' for input '%s'", i, test.expectedNTValid, foundNTValid, test.input)
			}
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
	}
}
