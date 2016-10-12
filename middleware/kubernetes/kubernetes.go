// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	dns_strings "github.com/miekg/coredns/middleware/pkg/strings"
	"github.com/miekg/coredns/middleware/proxy"

	"github.com/miekg/dns"
	"k8s.io/kubernetes/pkg/api"
	unversionedapi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	unversionedclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/labels"
)

// Kubernetes implements a middleware that connects to a Kubernetes cluster.
type Kubernetes struct {
	Next          middleware.Handler
	Zones         []string
	Proxy         proxy.Proxy // Proxy for looking up names during the resolution process
	APIEndpoint   string
	APICertAuth   string
	APIClientCert string
	APIClientKey  string
	APIConn       *dnsController
	ResyncPeriod  time.Duration
	NameTemplate  *nametemplate.Template
	Namespaces    []string
	LabelSelector *unversionedapi.LabelSelector
	Selector      *labels.Selector
}

func (k *Kubernetes) getClientConfig() (*restclient.Config, error) {
	// For a custom api server or running outside a k8s cluster
	// set URL in env.KUBERNETES_MASTER or set endpoint in Corefile
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}
	if len(k.APIEndpoint) > 0 {
		clusterinfo.Server = k.APIEndpoint
	}
	if len(k.APICertAuth) > 0 {
		clusterinfo.CertificateAuthority = k.APICertAuth
	}
	if len(k.APIClientCert) > 0 {
		authinfo.ClientCertificate = k.APIClientCert
	}
	if len(k.APIClientKey) > 0 {
		authinfo.ClientKey = k.APIClientKey
	}
	overrides.ClusterInfo = clusterinfo
	overrides.AuthInfo = authinfo
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return clientConfig.ClientConfig()
}

// InitKubeCache initializes a new Kubernetes cache.
// TODO(miek): is this correct?
func (k *Kubernetes) InitKubeCache() error {

	config, err := k.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := unversionedclient.New(config)
	if err != nil {
		log.Printf("[ERROR] Failed to create kubernetes notification controller: %v", err)
		return err
	}
	if k.LabelSelector == nil {
		log.Printf("[INFO] Kubernetes middleware configured without a label selector. No label-based filtering will be performed.")
	} else {
		var selector labels.Selector
		selector, err = unversionedapi.LabelSelectorAsSelector(k.LabelSelector)
		k.Selector = &selector
		if err != nil {
			log.Printf("[ERROR] Unable to create Selector for LabelSelector '%s'.Error was: %s", k.LabelSelector, err)
			return err
		}
		log.Printf("[INFO] Kubernetes middleware configured with the label selector '%s'. Only kubernetes objects matching this label selector will be exposed.", unversionedapi.FormatLabelSelector(k.LabelSelector))
	}
	k.APIConn = newdnsController(kubeClient, k.ResyncPeriod, k.Selector)

	return err
}

// getZoneForName returns the zone string that matches the name and a
// list of the DNS labels from name that are within the zone.
// For example, if "coredns.local" is a zone configured for the
// Kubernetes middleware, then getZoneForName("a.b.coredns.local")
// will return ("coredns.local", ["a", "b"]).
func (k *Kubernetes) getZoneForName(name string) (string, []string) {
	var zone string
	var serviceSegments []string

	for _, z := range k.Zones {
		if dns.IsSubDomain(z, name) {
			zone = z

			serviceSegments = dns.SplitDomainName(name)
			serviceSegments = serviceSegments[:len(serviceSegments)-dns.CountLabel(zone)]
			break
		}
	}

	return zone, serviceSegments
}

// Records looks up services in kubernetes.
// If exact is true, it will lookup just
// this name. This is used when find matches when completing SRV lookups
// for instance.
func (k *Kubernetes) Records(name string, exact bool) ([]msg.Service, error) {
	// TODO: refector this.
	// Right now NamespaceFromSegmentArray do not supports PRE queries
	ip := dnsutil.ExtractAddressFromReverse(name)
	if ip != "" {
		records := k.getServiceRecordForIP(ip, name)
		return records, nil
	}
	var (
		serviceName string
		namespace   string
		typeName    string
	)

	zone, serviceSegments := k.getZoneForName(name)

	// TODO: Implementation above globbed together segments for the serviceName if
	//       multiple segments remained. Determine how to do similar globbing using
	//		 the template-based implementation.
	namespace = k.NameTemplate.NamespaceFromSegmentArray(serviceSegments)
	serviceName = k.NameTemplate.ServiceFromSegmentArray(serviceSegments)
	typeName = k.NameTemplate.TypeFromSegmentArray(serviceSegments)

	if namespace == "" {
		err := errors.New("Parsing query string did not produce a namespace value. Assuming wildcard namespace.")
		log.Printf("[WARN] %v\n", err)
		namespace = "*"
	}

	if serviceName == "" {
		err := errors.New("Parsing query string did not produce a serviceName value. Assuming wildcard serviceName.")
		log.Printf("[WARN] %v\n", err)
		serviceName = "*"
	}

	nsWildcard := symbolContainsWildcard(namespace)
	serviceWildcard := symbolContainsWildcard(serviceName)

	// Abort if the namespace does not contain a wildcard, and namespace is not published per CoreFile
	// Case where namespace contains a wildcard is handled in Get(...) method.
	if (!nsWildcard) && (len(k.Namespaces) > 0) && (!dns_strings.StringInSlice(namespace, k.Namespaces)) {
		return nil, nil
	}

	k8sItems, err := k.Get(namespace, nsWildcard, serviceName, serviceWildcard)
	if err != nil {
		return nil, err
	}
	if k8sItems == nil {
		// Did not find item in k8s
		return nil, nil
	}

	records := k.getRecordsForServiceItems(k8sItems, nametemplate.NameValues{TypeName: typeName, ServiceName: serviceName, Namespace: namespace, Zone: zone})
	return records, nil
}

// TODO: assemble name from parts found in k8s data based on name template rather than reusing query string
func (k *Kubernetes) getRecordsForServiceItems(serviceItems []*api.Service, values nametemplate.NameValues) []msg.Service {
	var records []msg.Service

	for _, item := range serviceItems {
		clusterIP := item.Spec.ClusterIP

		// Create records by constructing record name from template...
		//values.Namespace = item.Metadata.Namespace
		//values.ServiceName = item.Metadata.Name
		//s := msg.Service{Host: g.NameTemplate.GetRecordNameFromNameValues(values)}
		//records = append(records, s)

		// Create records for each exposed port...
		for _, p := range item.Spec.Ports {
			s := msg.Service{Host: clusterIP, Port: int(p.Port)}
			records = append(records, s)
		}
	}

	return records
}

// Get performs the call to the Kubernetes http API.
func (k *Kubernetes) Get(namespace string, nsWildcard bool, servicename string, serviceWildcard bool) ([]*api.Service, error) {
	serviceList := k.APIConn.ServiceList()

	var resultItems []*api.Service

	for _, item := range serviceList {
		if symbolMatches(namespace, item.Namespace, nsWildcard) && symbolMatches(servicename, item.Name, serviceWildcard) {
			// If namespace has a wildcard, filter results against Corefile namespace list.
			// (Namespaces without a wildcard were filtered before the call to this function.)
			if nsWildcard && (len(k.Namespaces) > 0) && (!dns_strings.StringInSlice(item.Namespace, k.Namespaces)) {
				continue
			}
			resultItems = append(resultItems, item)
		}
	}

	return resultItems, nil
}

func symbolMatches(queryString string, candidateString string, wildcard bool) bool {
	result := false
	switch {
	case !wildcard:
		result = (queryString == candidateString)
	case queryString == "*":
		result = true
	case queryString == "any":
		result = true
	}
	return result
}

// kubernetesNameError checks if the error is ErrorCodeKeyNotFound from kubernetes.
func isKubernetesNameError(err error) bool {
	return false
}

func (k *Kubernetes) getServiceRecordForIP(ip, name string) []msg.Service {
	svcList, err := k.APIConn.svcLister.List(labels.Everything())
	if err != nil {
		return nil
	}
	for _, service := range svcList {
		if service.Spec.ClusterIP == ip {
			return []msg.Service{msg.Service{Host: ip}}
		}
	}

	return nil
}

// symbolContainsWildcard checks whether symbol contains a wildcard value
func symbolContainsWildcard(symbol string) bool {
	return (strings.Contains(symbol, "*") || (symbol == "any"))
}
