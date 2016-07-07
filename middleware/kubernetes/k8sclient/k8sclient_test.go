package k8sclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var validURLs = []string{
	"http://www.github.com",
	"http://www.github.com:8080",
	"http://8.8.8.8",
	"http://8.8.8.8:9090",
	"www.github.com:8080",
}

var invalidURLs = []string{
	"www.github.com",
	"8.8.8.8",
	"8.8.8.8:1010",
	"8.8`8.8",
}

func TestNewK8sConnector(t *testing.T) {
	var conn *K8sConnector
	var url string

	// Create with empty URL
	conn = nil
	url = ""

	conn = NewK8sConnector("")
	if conn == nil {
		t.Errorf("Expected K8sConnector instance. Instead got '%v'", conn)
	}
	url = conn.GetBaseURL()
	if url != defaultBaseURL {
		t.Errorf("Expected K8sConnector instance to be initialized with defaultBaseURL. Instead got '%v'", url)
	}

	// Create with valid URL
	for _, validURL := range validURLs {
		conn = nil
		url = ""

		conn = NewK8sConnector(validURL)
		if conn == nil {
			t.Errorf("Expected K8sConnector instance. Instead got '%v'", conn)
		}
		url = conn.GetBaseURL()
		if url != validURL {
			t.Errorf("Expected K8sConnector instance to be initialized with supplied url '%v'. Instead got '%v'", validURL, url)
		}
	}

	// Create with invalid URL
	for _, invalidURL := range invalidURLs {
		conn = nil
		url = ""

		conn = NewK8sConnector(invalidURL)
		if conn != nil {
			t.Errorf("Expected to not get K8sConnector instance. Instead got '%v'", conn)
			continue
		}
	}
}

func TestSetBaseURL(t *testing.T) {
	// SetBaseURL with valid URLs should work...
	for _, validURL := range validURLs {
		conn := NewK8sConnector(defaultBaseURL)
		err := conn.SetBaseURL(validURL)
		if err != nil {
			t.Errorf("Expected to receive nil, instead got error '%v'", err)
			continue
		}
		url := conn.GetBaseURL()
		if url != validURL {
			t.Errorf("Expected to connector url to be set to value '%v', instead set to '%v'", validURL, url)
			continue
		}
	}

	// SetBaseURL with invalid or non absolute URLs should not change state...
	for _, invalidURL := range invalidURLs {
		conn := NewK8sConnector(defaultBaseURL)
		originalURL := conn.GetBaseURL()

		err := conn.SetBaseURL(invalidURL)
		if err == nil {
			t.Errorf("Expected to receive an error value, instead got nil")
		}
		url := conn.GetBaseURL()
		if url != originalURL {
			t.Errorf("Expected base url to not change, instead it changed to '%v'", url)
		}
	}
}

func TestGetNamespaceList(t *testing.T) {
	// Set up a test http server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, namespaceListJsonData)
	}))
	defer testServer.Close()

	// Overwrite URL constructor to access testServer
	makeURL = func(parts []string) string {
		return testServer.URL
	}

	expectedNamespaces := []string{"default", "demo", "test"}
	apiConn := NewK8sConnector("")
	namespaceList, err := apiConn.GetNamespaceList()

	if err != nil {
		t.Errorf("Expected no error from from GetNamespaceList(), instead got %v", err)
	}

	if namespaceList == nil {
		t.Errorf("Expected data from GetNamespaceList(), instead got nil")
	}

	kind := namespaceList.Kind
	if kind != "NamespaceList" {
		t.Errorf("Expected data from GetNamespaceList() to have Kind='NamespaceList', instead got Kind='%v'", kind)
	}

	// Ensure correct number of namespaces found
	expectedCount := len(expectedNamespaces)
	namespaceCount := len(namespaceList.Items)
	if namespaceCount != expectedCount {
		t.Errorf("Expected '%v' namespaces from GetNamespaceList(), instead found '%v' namespaces", expectedCount, namespaceCount)
	}

	// Check that all expectedNamespaces are found in the parsed data
	for _, ns := range expectedNamespaces {
		found := false
		for _, item := range namespaceList.Items {
			if item.Metadata.Name == ns {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected '%v' namespace is not in the parsed data from GetServicesByNamespace()", ns)
		}
	}
}

func TestGetServiceList(t *testing.T) {
	// Set up a test http server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, serviceListJsonData)
	}))
	defer testServer.Close()

	// Overwrite URL constructor to access testServer
	makeURL = func(parts []string) string {
		return testServer.URL
	}

	expectedServices := []string{"kubernetes", "mynginx", "mywebserver"}
	apiConn := NewK8sConnector("")
	serviceList, err := apiConn.GetServiceList()

	if err != nil {
		t.Errorf("Expected no error from from GetNamespaceList(), instead got %v", err)
	}

	if serviceList == nil {
		t.Errorf("Expected data from GetServiceList(), instead got nil")
	}

	kind := serviceList.Kind
	if kind != "ServiceList" {
		t.Errorf("Expected data from GetServiceList() to have Kind='ServiceList', instead got Kind='%v'", kind)
	}

	// Ensure correct number of services found
	expectedCount := len(expectedServices)
	serviceCount := len(serviceList.Items)
	if serviceCount != expectedCount {
		t.Errorf("Expected '%v' services from GetServiceList(), instead found '%v' services", expectedCount, serviceCount)
	}

	// Check that all expectedServices are found in the parsed data
	for _, s := range expectedServices {
		found := false
		for _, item := range serviceList.Items {
			if item.Metadata.Name == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected '%v' service is not in the parsed data from GetServiceList()", s)
		}
	}
}

func TestGetServicesByNamespace(t *testing.T) {
	// Set up a test http server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, serviceListJsonData)
	}))
	defer testServer.Close()

	// Overwrite URL constructor to access testServer
	makeURL = func(parts []string) string {
		return testServer.URL
	}

	expectedNamespaces := []string{"default", "demo"}
	apiConn := NewK8sConnector("")
	servicesByNamespace, err := apiConn.GetServicesByNamespace()

	if err != nil {
		t.Errorf("Expected no error from from GetServicesByNamespace(), instead got %v", err)
	}

	// Ensure correct number of namespaces found
	expectedCount := len(expectedNamespaces)
	namespaceCount := len(servicesByNamespace)
	if namespaceCount != expectedCount {
		t.Errorf("Expected '%v' namespaces from GetServicesByNamespace(), instead found '%v' namespaces", expectedCount, namespaceCount)
	}

	// Check that all expectedNamespaces are found in the parsed data
	for _, ns := range expectedNamespaces {
		_, ok := servicesByNamespace[ns]
		if !ok {
			t.Errorf("Expected '%v' namespace is not in the parsed data from GetServicesByNamespace()", ns)
		}
	}
}

func TestGetResourceList(t *testing.T) {
	// Set up a test http server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, resourceListJsonData)
	}))
	defer testServer.Close()

	// Overwrite URL constructor to access testServer
	makeURL = func(parts []string) string {
		return testServer.URL
	}

	expectedResources := []string{"bindings",
		"componentstatuses",
		"configmaps",
		"endpoints",
		"events",
		"limitranges",
		"namespaces",
		"namespaces/finalize",
		"namespaces/status",
		"nodes",
		"nodes/proxy",
		"nodes/status",
		"persistentvolumeclaims",
		"persistentvolumeclaims/status",
		"persistentvolumes",
		"persistentvolumes/status",
		"pods",
		"pods/attach",
		"pods/binding",
		"pods/exec",
		"pods/log",
		"pods/portforward",
		"pods/proxy",
		"pods/status",
		"podtemplates",
		"replicationcontrollers",
		"replicationcontrollers/scale",
		"replicationcontrollers/status",
		"resourcequotas",
		"resourcequotas/status",
		"secrets",
		"serviceaccounts",
		"services",
		"services/proxy",
		"services/status",
	}
	apiConn := NewK8sConnector("")
	resourceList, err := apiConn.GetResourceList()

	if err != nil {
		t.Errorf("Expected no error from from GetResourceList(), instead got %v", err)
	}

	if resourceList == nil {
		t.Errorf("Expected data from GetResourceList(), instead got nil")
	}

	kind := resourceList.Kind
	if kind != "APIResourceList" {
		t.Errorf("Expected data from GetResourceList() to have Kind='ResourceList', instead got Kind='%v'", kind)
	}

	// Ensure correct number of resources found
	expectedCount := len(expectedResources)
	resourceCount := len(resourceList.Resources)
	if resourceCount != expectedCount {
		t.Errorf("Expected '%v' resources from GetResourceList(), instead found '%v' resources", expectedCount, resourceCount)
	}

	// Check that all expectedResources are found in the parsed data
	for _, r := range expectedResources {
		found := false
		for _, item := range resourceList.Resources {
			if item.Name == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected '%v' resource is not in the parsed data from GetResourceList()", r)
		}
	}
}

// Sample namespace data for kubernetes with 3 namespaces:
// "default", "demo", and "test".
const namespaceListJsonData string = `{
  "kind": "NamespaceList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/namespaces/",
    "resourceVersion": "121279"
  },
  "items": [
    {
      "metadata": {
        "name": "default",
        "selfLink": "/api/v1/namespaces/default",
        "uid": "fb1c92d1-2f39-11e6-b9db-0800279930f6",
        "resourceVersion": "6",
        "creationTimestamp": "2016-06-10T18:34:35Z"
      },
      "spec": {
        "finalizers": [
          "kubernetes"
        ]
      },
      "status": {
        "phase": "Active"
      }
    },
    {
      "metadata": {
        "name": "demo",
        "selfLink": "/api/v1/namespaces/demo",
        "uid": "73be8ffd-2f3a-11e6-b9db-0800279930f6",
        "resourceVersion": "111",
        "creationTimestamp": "2016-06-10T18:37:57Z"
      },
      "spec": {
        "finalizers": [
          "kubernetes"
        ]
      },
      "status": {
        "phase": "Active"
      }
    },
    {
      "metadata": {
        "name": "test",
        "selfLink": "/api/v1/namespaces/test",
        "uid": "c0be05fa-3352-11e6-b9db-0800279930f6",
        "resourceVersion": "121276",
        "creationTimestamp": "2016-06-15T23:41:59Z"
      },
      "spec": {
        "finalizers": [
          "kubernetes"
        ]
      },
      "status": {
        "phase": "Active"
      }
    }
  ]
}`

// Sample service data for kubernetes with 3 services:
//	* "kubernetes" (in "default" namespace)
//	* "mynginx" (in "demo" namespace)
//	* "webserver" (in "demo" namespace)
const serviceListJsonData string = `
{
  "kind": "ServiceList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/services",
    "resourceVersion": "147965"
  },
  "items": [
    {
      "metadata": {
        "name": "kubernetes",
        "namespace": "default",
        "selfLink": "/api/v1/namespaces/default/services/kubernetes",
        "uid": "fb1cb0d3-2f39-11e6-b9db-0800279930f6",
        "resourceVersion": "7",
        "creationTimestamp": "2016-06-10T18:34:35Z",
        "labels": {
          "component": "apiserver",
          "provider": "kubernetes"
        }
      },
      "spec": {
        "ports": [
          {
            "name": "https",
            "protocol": "TCP",
            "port": 443,
            "targetPort": 443
          }
        ],
        "clusterIP": "10.0.0.1",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    },
    {
      "metadata": {
        "name": "mynginx",
        "namespace": "demo",
        "selfLink": "/api/v1/namespaces/demo/services/mynginx",
        "uid": "93c117ac-2f3a-11e6-b9db-0800279930f6",
        "resourceVersion": "147",
        "creationTimestamp": "2016-06-10T18:38:51Z",
        "labels": {
          "run": "mynginx"
        }
      },
      "spec": {
        "ports": [
          {
            "protocol": "TCP",
            "port": 80,
            "targetPort": 80
          }
        ],
        "selector": {
          "run": "mynginx"
        },
        "clusterIP": "10.0.0.132",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    },
    {
      "metadata": {
        "name": "mywebserver",
        "namespace": "demo",
        "selfLink": "/api/v1/namespaces/demo/services/mywebserver",
        "uid": "aed62187-33e5-11e6-a224-0800279930f6",
        "resourceVersion": "138185",
        "creationTimestamp": "2016-06-16T17:13:45Z",
        "labels": {
          "run": "mywebserver"
        }
      },
      "spec": {
        "ports": [
          {
            "protocol": "TCP",
            "port": 443,
            "targetPort": 443
          }
        ],
        "selector": {
          "run": "mywebserver"
        },
        "clusterIP": "10.0.0.63",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    }
  ]
}
`

// Sample resource data for kubernetes.
const resourceListJsonData string = `{
  "kind": "APIResourceList",
  "groupVersion": "v1",
  "resources": [
    {
      "name": "bindings",
      "namespaced": true,
      "kind": "Binding"
    },
    {
      "name": "componentstatuses",
      "namespaced": false,
      "kind": "ComponentStatus"
    },
    {
      "name": "configmaps",
      "namespaced": true,
      "kind": "ConfigMap"
    },
    {
      "name": "endpoints",
      "namespaced": true,
      "kind": "Endpoints"
    },
    {
      "name": "events",
      "namespaced": true,
      "kind": "Event"
    },
    {
      "name": "limitranges",
      "namespaced": true,
      "kind": "LimitRange"
    },
    {
      "name": "namespaces",
      "namespaced": false,
      "kind": "Namespace"
    },
    {
      "name": "namespaces/finalize",
      "namespaced": false,
      "kind": "Namespace"
    },
    {
      "name": "namespaces/status",
      "namespaced": false,
      "kind": "Namespace"
    },
    {
      "name": "nodes",
      "namespaced": false,
      "kind": "Node"
    },
    {
      "name": "nodes/proxy",
      "namespaced": false,
      "kind": "Node"
    },
    {
      "name": "nodes/status",
      "namespaced": false,
      "kind": "Node"
    },
    {
      "name": "persistentvolumeclaims",
      "namespaced": true,
      "kind": "PersistentVolumeClaim"
    },
    {
      "name": "persistentvolumeclaims/status",
      "namespaced": true,
      "kind": "PersistentVolumeClaim"
    },
    {
      "name": "persistentvolumes",
      "namespaced": false,
      "kind": "PersistentVolume"
    },
    {
      "name": "persistentvolumes/status",
      "namespaced": false,
      "kind": "PersistentVolume"
    },
    {
      "name": "pods",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/attach",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/binding",
      "namespaced": true,
      "kind": "Binding"
    },
    {
      "name": "pods/exec",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/log",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/portforward",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/proxy",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/status",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "podtemplates",
      "namespaced": true,
      "kind": "PodTemplate"
    },
    {
      "name": "replicationcontrollers",
      "namespaced": true,
      "kind": "ReplicationController"
    },
    {
      "name": "replicationcontrollers/scale",
      "namespaced": true,
      "kind": "Scale"
    },
    {
      "name": "replicationcontrollers/status",
      "namespaced": true,
      "kind": "ReplicationController"
    },
    {
      "name": "resourcequotas",
      "namespaced": true,
      "kind": "ResourceQuota"
    },
    {
      "name": "resourcequotas/status",
      "namespaced": true,
      "kind": "ResourceQuota"
    },
    {
      "name": "secrets",
      "namespaced": true,
      "kind": "Secret"
    },
    {
      "name": "serviceaccounts",
      "namespaced": true,
      "kind": "ServiceAccount"
    },
    {
      "name": "services",
      "namespaced": true,
      "kind": "Service"
    },
    {
      "name": "services/proxy",
      "namespaced": true,
      "kind": "Service"
    },
    {
      "name": "services/status",
      "namespaced": true,
      "kind": "Service"
    }
  ]
}`
