package k8sclient

import (
    "encoding/json"
    "net/http"
)


func getJson(url string, target interface{}) error {
    r, err := http.Get(url)
    if err != nil {
        return err
    }
    defer r.Body.Close()

    return json.NewDecoder(r.Body).Decode(target)
}


// Kubernetes Resource List
type ResourceList struct {
    Kind         string `json:"kind"`
    GroupVersion string `json:"groupVersion"`
    Resources    []resource `json:"resources"`
}

type resource struct {
    Name         string `json:"name"`
    Namespaced   bool   `json:"namespaced"`
    Kind         string `json:"kind"`
}


// Kubernetes NamespaceList
type NamespaceList struct {
    Kind         string          `json:"kind"`
    APIVersion   string          `json:"apiVersion"`
    Metadata     apiListMetadata  `json:"metadata"`
    Items        []nsItems       `json:"items"`   
}

type apiListMetadata struct {
    SelfLink         string `json:"selfLink"`
    resourceVersion  string `json:"resourceVersion"`
}

type nsItems struct {
    Metadata   nsMetadata  `json:"metadata"`
    Spec       nsSpec      `json:"spec"`
    Status     nsStatus    `json:"status"`
}

type nsMetadata struct {
    Name              string  `json:"name"`
    SelfLink          string  `json:"selfLink"`
    Uid               string  `json:"uid"`
    ResourceVersion   string  `json:"resourceVersion"`
    CreationTimestamp string  `json:"creationTimestamp"`
}

type nsSpec struct {
    Finalizers []string `json:"finalizers"`
}

type nsStatus struct {
    Phase string `json:"phase"`
}


// Kubernetes ServiceList
type ServiceList struct {
    Kind         string          `json:"kind"`
    APIVersion   string          `json:"apiVersion"`
    Metadata     apiListMetadata  `json:"metadata"`
    Items        []ServiceItem    `json:"items"`   
}

type ServiceItem struct {
    Metadata   serviceMetadata  `json:"metadata"`
    Spec       serviceSpec      `json:"spec"`
//    Status     serviceStatus    `json:"status"`
}

type serviceMetadata struct {
    Name              string  `json:"name"`
    Namespace         string  `json:"namespace"`
    SelfLink          string  `json:"selfLink"`
    Uid               string  `json:"uid"`
    ResourceVersion   string  `json:"resourceVersion"`
    CreationTimestamp string  `json:"creationTimestamp"`
    // labels
}

type serviceSpec struct {
    Ports           []servicePort `json:"ports"`
    ClusterIP       string        `json:"clusterIP"`
    Type            string        `json:"type"`
    SessionAffinity string        `json:"sessionAffinity"`
}

type servicePort struct {
    Name            string        `json:"name"`
    Protocol        string        `json:"protocol"`
    Port            int           `json:"port"`
    TargetPort      int           `json:"targetPort"`
}

type serviceStatus struct {
    LoadBalancer string `json:"loadBalancer"`
}
