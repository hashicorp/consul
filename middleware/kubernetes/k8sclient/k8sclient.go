package k8sclient

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// API strings
const (
	apiBase       = "/api/v1"
	apiNamespaces = "/namespaces"
	apiServices   = "/services"
)

// Defaults
const (
	defaultBaseURL = "http://localhost:8080"
)

type K8sConnector struct {
	baseURL string
}

func (c *K8sConnector) SetBaseURL(u string) error {
	url, error := url.Parse(u)

	if error != nil {
		return error
	}

	if !url.IsAbs() {
		return errors.New("k8sclient: Kubernetes endpoint url must be an absolute URL")
	}

	c.baseURL = url.String()
	return nil
}

func (c *K8sConnector) GetBaseURL() string {
	return c.baseURL
}

// URL constructor separated from code to support dependency injection
// for unit tests.
var makeURL = func(parts []string) string {
	return strings.Join(parts, "")
}

func (c *K8sConnector) GetResourceList() (*ResourceList, error) {
	resources := new(ResourceList)

	url := makeURL([]string{c.baseURL, apiBase})
	err := parseJson(url, resources)
	// TODO: handle no response from k8s
	if err != nil {
		fmt.Printf("[ERROR] Response from kubernetes API for GetResourceList() is: %v\n", err)
		return nil, err
	}

	return resources, nil
}

func (c *K8sConnector) GetNamespaceList() (*NamespaceList, error) {
	namespaces := new(NamespaceList)

	url := makeURL([]string{c.baseURL, apiBase, apiNamespaces})
	err := parseJson(url, namespaces)
	if err != nil {
		fmt.Printf("[ERROR] Response from kubernetes API for GetNamespaceList() is: %v\n", err)
		return nil, err
	}

	return namespaces, nil
}

func (c *K8sConnector) GetServiceList() (*ServiceList, error) {
	services := new(ServiceList)

	url := makeURL([]string{c.baseURL, apiBase, apiServices})
	err := parseJson(url, services)
	// TODO: handle no response from k8s
	if err != nil {
		fmt.Printf("[ERROR] Response from kubernetes API for GetServiceList() is: %v\n", err)
		return nil, err
	}

	return services, nil
}

// GetServicesByNamespace returns a map of
// namespacename :: [ kubernetesServiceItem ]
func (c *K8sConnector) GetServicesByNamespace() (map[string][]ServiceItem, error) {

	items := make(map[string][]ServiceItem)

	k8sServiceList, err := c.GetServiceList()

	if err != nil {
		fmt.Printf("[ERROR] Getting service list produced error: %v", err)
		return nil, err
	}

	// TODO: handle no response from k8s
	if k8sServiceList == nil {
		return nil, nil
	}

	k8sItemList := k8sServiceList.Items

	for _, i := range k8sItemList {
		namespace := i.Metadata.Namespace
		items[namespace] = append(items[namespace], i)
	}

	return items, nil
}

func NewK8sConnector(baseURL string) *K8sConnector {
	k := new(K8sConnector)

	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	err := k.SetBaseURL(baseURL)
	if err != nil {
		return nil
	}

	return k
}
