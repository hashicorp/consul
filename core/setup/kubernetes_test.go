package setup

import (
	"strings"
	"testing"
)

/*
kubernetes coredns.local {
        # Use url for k8s API endpoint
        endpoint http://localhost:8080
        # Assemble k8s record names with the template
        template {service}.{namespace}.{zone}
        # Only expose the k8s namespace "demo"
        #namespaces demo
    }
*/

func TestKubernetesParse(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedErrContent string // substring from the expected error. Empty for positive cases.
		expectedZoneCount  int    // expected count of defined zones. '-1' for negative cases.
		expectedNTValid    bool   // NameTemplate to be initialized and valid
		expectedNSCount    int    // expected count of namespaces. '-1' for negative cases.
	}{
		// positive
		// TODO: not specifiying a zone maybe should error out.
		{
			`kubernetes`,
			false,
			"",
			0,
			true,
			0,
		},
		{
			`kubernetes coredns.local`,
			false,
			"",
			1,
			true,
			0,
		},
		{
			`kubernetes coredns.local test.local`,
			false,
			"",
			2,
			true,
			0,
		},
		{
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
			`kubernetes coredns.local {
	namespaces demo test
}`,
			false,
			"",
			1,
			true,
			2,
		},

		// negative
		{
			`kubernetes coredns.local {
    endpoint
}`,
			true,
			"Wrong argument count or unexpected line ending after 'endpoint'",
			-1,
			true,
			-1,
		},
		// No template provided for template line.
		{
			`kubernetes coredns.local {
    template
}`,
			true,
			"",
			-1,
			false,
			-1,
		},
		// Invalid template provided
		{
			`kubernetes coredns.local {
    template {namespace}.{zone}
}`,
			true,
			"",
			-1,
			false,
			-1,
		},
		/*
			 		// No valid provided for namespaces
			   		{
			   			`kubernetes coredns.local {
			     namespaces
			}`,
			   			true,
			   			"",
			   			-1,
						true,
			   			-1,
			   		},
		*/
	}

	for i, test := range tests {
		c := NewTestController(test.input)
		k8sController, err := kubernetesParse(c)
		t.Logf("i: %v\n", i)
		t.Logf("controller: %v\n", k8sController)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error, but found one for input '%s'. Error was: '%v'", i, test.input, err)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
			}

			if test.shouldErr && (len(test.expectedErrContent) < 1) {
				t.Fatalf("Test %d: Test marked as expecting an error, but no expectedErrContent provided for input '%s'. Error was: '%v'", i, test.input, err)
			}

			if test.shouldErr && (test.expectedZoneCount >= 0) {
				t.Fatalf("Test %d: Test marked as expecting an error, but provides value for expectedZoneCount!=-1 for input '%s'. Error was: '%v'", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}

			return
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

	}
}
