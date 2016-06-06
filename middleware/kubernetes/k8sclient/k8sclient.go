package k8sclient

import (
//    "fmt"
    "net/url"
)

// API strings
const (
    apiBase       = "/api/v1"
    apiNamespaces = "/namespaces"
    apiServices   = "/services"
)

// Defaults
const (
    defaultBaseUrl = "http://localhost:8080"
)


type K8sConnector struct {
    baseUrl string
}

func (c *K8sConnector) SetBaseUrl(u string) error {
    validUrl, error := url.Parse(u)

    if error != nil {
        return error
    }
    c.baseUrl = validUrl.String()

    return nil
}

func (c *K8sConnector) GetBaseUrl() string {
    return c.baseUrl
}


func (c *K8sConnector) GetResourceList() *ResourceList {
    resources := new(ResourceList)
    
    error := getJson((c.baseUrl + apiBase), resources)
    if error != nil {
        return nil
    }

    return resources
}


func (c *K8sConnector) GetNamespaceList() *NamespaceList {
    namespaces := new(NamespaceList)

    error := getJson((c.baseUrl + apiBase + apiNamespaces), namespaces)
    if error != nil {
        return nil
    }

    return namespaces
}


func (c *K8sConnector) GetServiceList() *ServiceList {
    services := new(ServiceList)

    error := getJson((c.baseUrl + apiBase + apiServices), services)
    if error != nil {
        return nil
    }

    return services
}


func (c *K8sConnector) GetServicesByNamespace() map[string][]ServiceItem {
     // GetServicesByNamespace returns a map of namespacename :: [ kubernetesServiceItem ]

    items := make(map[string][]ServiceItem)

    k8sServiceList := c.GetServiceList()
    k8sItemList := k8sServiceList.Items

    for _, i := range k8sItemList {
        namespace := i.Metadata.Namespace
        items[namespace] = append(items[namespace], i)
    }

    return items
}


func (c *K8sConnector) GetServiceItemInNamespace(namespace string, servicename string) *ServiceItem {
    // GetServiceItemInNamespace returns the ServiceItem that matches servicename in the namespace

    itemMap := c.GetServicesByNamespace()

    // TODO: Handle case where namesapce == nil

    for _, x := range itemMap[namespace] {
        if x.Metadata.Name == servicename {
            return &x
        }
    }

    // No matching item found in namespace
    return nil
}


func NewK8sConnector(baseurl string) *K8sConnector {
    k := new(K8sConnector)
    k.SetBaseUrl(baseurl)

    return k
}
