package kubernetes

import (
	"testing"
)

func TestFilteredNamespaceExists(t *testing.T) {
	tests := []struct {
		expected             bool
		kubernetesNamespaces map[string]struct{}
		testNamespace        string
	}{
		{true, map[string]struct{}{}, "foobar"},
		{false, map[string]struct{}{}, "nsnoexist"},
	}

	k := Kubernetes{}
	k.APIConn = &APIConnServeTest{}
	for i, test := range tests {
		k.Namespaces = test.kubernetesNamespaces
		actual := k.filteredNamespaceExists(test.testNamespace)
		if actual != test.expected {
			t.Errorf("Test %d failed. Filtered namespace %s was expected to exist", i, test.testNamespace)
		}
	}
}

func TestNamespaceExposed(t *testing.T) {
	tests := []struct {
		expected             bool
		kubernetesNamespaces map[string]struct{}
		testNamespace        string
	}{
		{true, map[string]struct{}{"foobar": {}}, "foobar"},
		{false, map[string]struct{}{"foobar": {}}, "nsnoexist"},
		{true, map[string]struct{}{}, "foobar"},
		{true, map[string]struct{}{}, "nsnoexist"},
	}

	k := Kubernetes{}
	k.APIConn = &APIConnServeTest{}
	for i, test := range tests {
		k.Namespaces = test.kubernetesNamespaces
		actual := k.configuredNamespace(test.testNamespace)
		if actual != test.expected {
			t.Errorf("Test %d failed. Namespace %s was expected to be exposed", i, test.testNamespace)
		}
	}
}

func TestNamespaceValid(t *testing.T) {
	tests := []struct {
		expected             bool
		kubernetesNamespaces map[string]struct{}
		testNamespace        string
	}{
		{true, map[string]struct{}{"foobar": {}}, "foobar"},
		{false, map[string]struct{}{"foobar": {}}, "nsnoexist"},
		{true, map[string]struct{}{}, "foobar"},
		{false, map[string]struct{}{}, "nsnoexist"},
	}

	k := Kubernetes{}
	k.APIConn = &APIConnServeTest{}
	for i, test := range tests {
		k.Namespaces = test.kubernetesNamespaces
		actual := k.namespaceExposed(test.testNamespace)
		if actual != test.expected {
			t.Errorf("Test %d failed. Namespace %s was expected to be valid", i, test.testNamespace)
		}
	}
}
