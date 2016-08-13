// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/kubernetes/msg"
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"
	"github.com/miekg/coredns/middleware/kubernetes/util"
	"github.com/miekg/coredns/middleware/proxy"

	"github.com/miekg/dns"
	"k8s.io/kubernetes/pkg/api"
	unversionedapi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	unversionedclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

type Kubernetes struct {
	Next          middleware.Handler
	Zones         []string
	Proxy         proxy.Proxy // Proxy for looking up names during the resolution process
	APIEndpoint   string
	APIConn       *dnsController
	ResyncPeriod  time.Duration
	NameTemplate  *nametemplate.NameTemplate
	Namespaces    []string
	LabelSelector *unversionedapi.LabelSelector
	Selector      *labels.Selector 
}

func (g *Kubernetes) StartKubeCache() error {
	// For a custom api server or running outside a k8s cluster
	// set URL in env.KUBERNETES_MASTER or set endpoint in Corefile
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if len(g.APIEndpoint) > 0 {
		overrides.ClusterInfo = clientcmdapi.Cluster{Server: g.APIEndpoint}
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		log.Printf("[debug] error connecting to the client: %v", err)
		return err
	}
	kubeClient, err := unversionedclient.New(config)

	if err != nil {
		log.Printf("[ERROR] Failed to create kubernetes notification controller: %v", err)
		return err
	}
	if g.LabelSelector == nil {
		log.Printf("[INFO] Kubernetes middleware configured without a label selector. No label-based filtering will be operformed.")
	} else {
        var selector labels.Selector
		selector, err = unversionedapi.LabelSelectorAsSelector(g.LabelSelector)
        g.Selector = &selector
        if err != nil {
            log.Printf("[ERROR] Unable to create Selector for LabelSelector '%s'.Error was: %s", g.LabelSelector, err)
            return err
        }
		log.Printf("[INFO] Kubernetes middleware configured with the label selector '%s'. Only kubernetes objects matching this label selector will be exposed.", unversionedapi.FormatLabelSelector(g.LabelSelector))
	}
	log.Printf("[debug] Starting kubernetes middleware with k8s API resync period: %s", g.ResyncPeriod)
	g.APIConn = newdnsController(kubeClient, g.ResyncPeriod, g.Selector)

	go g.APIConn.Run()

	return err
}

// getZoneForName returns the zone string that matches the name and a
// list of the DNS labels from name that are within the zone.
// For example, if "coredns.local" is a zone configured for the
// Kubernetes middleware, then getZoneForName("a.b.coredns.local")
// will return ("coredns.local", ["a", "b"]).
func (g *Kubernetes) getZoneForName(name string) (string, []string) {
	var zone string
	var serviceSegments []string

	for _, z := range g.Zones {
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
func (g *Kubernetes) Records(name string, exact bool) ([]msg.Service, error) {
	// TODO: refector this.
	// Right now GetNamespaceFromSegmentArray do not supports PRE queries
	if strings.HasSuffix(name, arpaSuffix) {
		ip, _ := extractIP(name)
		records := g.getServiceRecordForIP(ip, name)
		return records, nil
	}
	var (
		serviceName string
		namespace   string
		typeName    string
	)

	log.Printf("[debug] enter Records('%v', '%v')\n", name, exact)
	zone, serviceSegments := g.getZoneForName(name)

	// TODO: Implementation above globbed together segments for the serviceName if
	//       multiple segments remained. Determine how to do similar globbing using
	//		 the template-based implementation.
	namespace = g.NameTemplate.GetNamespaceFromSegmentArray(serviceSegments)
	serviceName = g.NameTemplate.GetServiceFromSegmentArray(serviceSegments)
	typeName = g.NameTemplate.GetTypeFromSegmentArray(serviceSegments)

	if namespace == "" {
		err := errors.New("Parsing query string did not produce a namespace value. Assuming wildcard namespace.")
		log.Printf("[WARN] %v\n", err)
		namespace = util.WildcardStar
	}

	if serviceName == "" {
		err := errors.New("Parsing query string did not produce a serviceName value. Assuming wildcard serviceName.")
		log.Printf("[WARN] %v\n", err)
		serviceName = util.WildcardStar
	}

	log.Printf("[debug] published namespaces: %v\n", g.Namespaces)

	log.Printf("[debug] exact: %v\n", exact)
	log.Printf("[debug] zone: %v\n", zone)
	log.Printf("[debug] servicename: %v\n", serviceName)
	log.Printf("[debug] namespace: %v\n", namespace)
	log.Printf("[debug] typeName: %v\n", typeName)
	log.Printf("[debug] APIconn: %v\n", g.APIConn)

	nsWildcard := util.SymbolContainsWildcard(namespace)
	serviceWildcard := util.SymbolContainsWildcard(serviceName)

	// Abort if the namespace does not contain a wildcard, and namespace is not published per CoreFile
	// Case where namespace contains a wildcard is handled in Get(...) method.
	if (!nsWildcard) && (len(g.Namespaces) > 0) && (!util.StringInSlice(namespace, g.Namespaces)) {
		log.Printf("[debug] Namespace '%v' is not published by Corefile\n", namespace)
		return nil, nil
	}

	log.Printf("before g.Get(namespace, nsWildcard, serviceName, serviceWildcard): %v %v %v %v", namespace, nsWildcard, serviceName, serviceWildcard)
	k8sItems, err := g.Get(namespace, nsWildcard, serviceName, serviceWildcard)
	log.Printf("[debug] k8s items: %v\n", k8sItems)
	if err != nil {
		log.Printf("[ERROR] Got error while looking up ServiceItems. Error is: %v\n", err)
		return nil, err
	}
	if k8sItems == nil {
		// Did not find item in k8s
		return nil, nil
	}

	records := g.getRecordsForServiceItems(k8sItems, nametemplate.NameValues{TypeName: typeName, ServiceName: serviceName, Namespace: namespace, Zone: zone})
	return records, nil
}

// TODO: assemble name from parts found in k8s data based on name template rather than reusing query string
func (g *Kubernetes) getRecordsForServiceItems(serviceItems []api.Service, values nametemplate.NameValues) []msg.Service {
	var records []msg.Service

	for _, item := range serviceItems {
		clusterIP := item.Spec.ClusterIP
		log.Printf("[debug] clusterIP: %v\n", clusterIP)

		// Create records by constructing record name from template...
		//values.Namespace = item.Metadata.Namespace
		//values.ServiceName = item.Metadata.Name
		//s := msg.Service{Host: g.NameTemplate.GetRecordNameFromNameValues(values)}
		//records = append(records, s)

		// Create records for each exposed port...
		for _, p := range item.Spec.Ports {
			log.Printf("[debug]    port: %v\n", p.Port)
			s := msg.Service{Host: clusterIP, Port: int(p.Port)}
			records = append(records, s)
		}
	}

	log.Printf("[debug] records from getRecordsForServiceItems(): %v\n", records)
	return records
}

// Get performs the call to the Kubernetes http API.
func (g *Kubernetes) Get(namespace string, nsWildcard bool, servicename string, serviceWildcard bool) ([]api.Service, error) {
	serviceList := g.APIConn.GetServiceList()

	/* TODO: Remove?
	if err != nil {
		log.Printf("[ERROR] Getting service list produced error: %v", err)
		return nil, err
	}
	*/

	var resultItems []api.Service

	for _, item := range serviceList.Items {
		if symbolMatches(namespace, item.Namespace, nsWildcard) && symbolMatches(servicename, item.Name, serviceWildcard) {
			// If namespace has a wildcard, filter results against Corefile namespace list.
			// (Namespaces without a wildcard were filtered before the call to this function.)
			if nsWildcard && (len(g.Namespaces) > 0) && (!util.StringInSlice(item.Namespace, g.Namespaces)) {
				log.Printf("[debug] Namespace '%v' is not published by Corefile\n", item.Namespace)
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
	case queryString == util.WildcardStar:
		result = true
	case queryString == util.WildcardAny:
		result = true
	}
	return result
}

// kubernetesNameError checks if the error is ErrorCodeKeyNotFound from kubernetes.
func isKubernetesNameError(err error) bool {
	return false
}

func (g *Kubernetes) getServiceRecordForIP(ip, name string) []msg.Service {
	svcList, err := g.APIConn.svcLister.List()
	if err != nil {
		return nil
	}

	for _, service := range svcList.Items {
		if service.Spec.ClusterIP == ip {
			return []msg.Service{msg.Service{Host: ip}}
		}
	}

	return nil
}

const (
	priority   = 10  // default priority when nothing is set
	ttl        = 300 // default ttl when nothing is set
	minTtl     = 60
	hostmaster = "hostmaster"
	k8sTimeout = 5 * time.Second
)
