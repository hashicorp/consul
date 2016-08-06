package setup

import (
	"strings"
	"testing"
)

func TestKubernetesParse(t *testing.T) {
	tests := []struct {
		description        string
		input              string
		shouldErr          bool
		expectedErrContent string // substring from the expected error. Empty for positive cases.
		expectedZoneCount  int    // expected count of defined zones.
		expectedNTValid    bool   // NameTemplate to be initialized and valid
		expectedNSCount    int    // expected count of namespaces.
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
		},
		{
			"kubernetes keyword with multiple zones",
			`kubernetes coredns.local test.local`,
			false,
			"",
			2,
			true,
			0,
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
		},
		{
			"fully specified valid config",
			`kubernetes coredns.local test.local {
	endpoint http://localhost:8080
	template {service}.{namespace}.{zone}
	namespaces demo test
}`,
			false,
			"",
			2,
			true,
			2,
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
		},
		{
			"kubernetes keyword without a zone",
			`kubernetes`,
			true,
			"Zone name must be provided for kubernetes middleware",
			-1,
			true,
			0,
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
		},
	}

	t.Logf("Parser test cases count: %v", len(tests))
	for i, test := range tests {
		c := NewTestController(test.input)
		k8sController, err := kubernetesParse(c)
		t.Logf("setup test: %2v -- %v\n", i, test.description)
		//t.Logf("controller: %v\n", k8sController)

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
			t.Logf("k8sController is: %v", k8sController)
			t.Logf("k8sController.Namespaces is: %v", k8sController.Namespaces)
		}
	}
}
