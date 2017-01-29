// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/dnsutil"
	dnsstrings "github.com/miekg/coredns/middleware/pkg/strings"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	unversionedapi "k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/labels"
	"k8s.io/client-go/1.5/rest"
	"k8s.io/client-go/1.5/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/1.5/tools/clientcmd/api"
)

// Kubernetes implements a middleware that connects to a Kubernetes cluster.
type Kubernetes struct {
	Next          middleware.Handler
	Zones         []string
	primaryZone   int
	Proxy         proxy.Proxy // Proxy for looking up names during the resolution process
	APIEndpoint   string
	APICertAuth   string
	APIClientCert string
	APIClientKey  string
	APIConn       *dnsController
	ResyncPeriod  time.Duration
	Namespaces    []string
	LabelSelector *unversionedapi.LabelSelector
	Selector      *labels.Selector
	PodMode       string
}

const (
	// PodModeDisabled is the default value where pod requests are ignored
	PodModeDisabled = "disabled"
	// PodModeVerified is where Pod requests are answered only if they exist
	PodModeVerified = "verified"
	// PodModeInsecure is where pod requests are answered without verfying they exist
	PodModeInsecure = "insecure"
	// DNSSchemaVersion is the schema version: https://github.com/kubernetes/dns/blob/master/docs/specification.md
	DNSSchemaVersion = "1.0.0"
)

type endpoint struct {
	addr api.EndpointAddress
	port api.EndpointPort
}

type service struct {
	name      string
	namespace string
	addr      string
	ports     []api.ServicePort
	endpoints []endpoint
}

type pod struct {
	name      string
	namespace string
	addr      string
}

type recordRequest struct {
	port, protocol, endpoint, service, namespace, typeName, zone string
}

var errNoItems = errors.New("no items found")
var errNsNotExposed = errors.New("namespace is not exposed")
var errInvalidRequest = errors.New("invalid query name")

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(state request.Request, exact bool, opt middleware.Options) ([]msg.Service, []msg.Service, error) {

	r, e := k.parseRequest(state.Name(), state.Type())
	if e != nil {
		return nil, nil, e
	}

	switch state.Type() {
	case "A", "SRV":
		s, e := k.Records(r)
		return s, nil, e // Haven't implemented debug queries yet.
	case "TXT":
		s, e := k.recordsForTXT(r)
		return s, nil, e
	}
	return nil, nil, nil
}

func (k *Kubernetes) recordsForTXT(r recordRequest) ([]msg.Service, error) {
	switch r.typeName {
	case "dns-version":
		s := msg.Service{
			Text: DNSSchemaVersion,
			TTL:  28800,
			Key:  msg.Path(r.typeName+"."+r.zone, "coredns")}
		return []msg.Service{s}, nil
	}
	return nil, nil
}

// PrimaryZone will return the first non-reverse zone being handled by this middleware
func (k *Kubernetes) PrimaryZone() string {
	return k.Zones[k.primaryZone]
}

// Reverse implements the ServiceBackend interface.
func (k *Kubernetes) Reverse(state request.Request, exact bool, opt middleware.Options) ([]msg.Service, []msg.Service, error) {
	ip := dnsutil.ExtractAddressFromReverse(state.Name())
	if ip == "" {
		return nil, nil, nil
	}

	records := k.getServiceRecordForIP(ip, state.Name())
	return records, nil, nil
}

// Lookup implements the ServiceBackend interface.
func (k *Kubernetes) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Proxy.Lookup(state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (k *Kubernetes) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest
}

// Debug implements the ServiceBackend interface.
func (k *Kubernetes) Debug() string {
	return "debug"
}

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	// For a custom api server or running outside a k8s cluster
	// set URL in env.KUBERNETES_MASTER or set endpoint in Corefile
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}
	if len(k.APIEndpoint) > 0 {
		clusterinfo.Server = k.APIEndpoint
	} else {
		cc, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return cc, err
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
func (k *Kubernetes) InitKubeCache() error {

	config, err := k.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Failed to create kubernetes notification controller: %v", err)
	}

	if k.LabelSelector != nil {
		var selector labels.Selector
		selector, err = unversionedapi.LabelSelectorAsSelector(k.LabelSelector)
		k.Selector = &selector
		if err != nil {
			return fmt.Errorf("Unable to create Selector for LabelSelector '%s'.Error was: %s", k.LabelSelector, err)
		}
	}

	if k.LabelSelector == nil {
		log.Printf("[INFO] Kubernetes middleware configured without a label selector. No label-based filtering will be performed.")
	} else {
		log.Printf("[INFO] Kubernetes middleware configured with the label selector '%s'. Only kubernetes objects matching this label selector will be exposed.", unversionedapi.FormatLabelSelector(k.LabelSelector))
	}

	k.APIConn = newdnsController(kubeClient, k.ResyncPeriod, k.Selector, k.PodMode == PodModeVerified)

	return err
}

func (k *Kubernetes) parseRequest(lowerCasedName, qtype string) (r recordRequest, err error) {
	// 3 Possible cases
	//   SRV Request: _port._protocol.service.namespace.type.zone
	//   A Request (endpoint): endpoint.service.namespace.type.zone
	//   A Request (service): service.namespace.type.zone

	// separate zone from rest of lowerCasedName
	var segs []string
	for _, z := range k.Zones {
		if dns.IsSubDomain(z, lowerCasedName) {
			r.zone = z

			segs = dns.SplitDomainName(lowerCasedName)
			segs = segs[:len(segs)-dns.CountLabel(r.zone)]
			break
		}
	}
	if r.zone == "" {
		return r, errors.New("zone not found")
	}

	offset := 0
	if qtype == "SRV" {
		if len(segs) != 5 {
			return r, errInvalidRequest
		}
		// This is a SRV style request, get first two elements as port and
		// protocol, stripping leading underscores if present.
		if segs[0][0] == '_' {
			r.port = segs[0][1:]
		} else {
			r.port = segs[0]
			if !symbolContainsWildcard(r.port) {
				return r, errInvalidRequest
			}
		}
		if segs[1][0] == '_' {
			r.protocol = segs[1][1:]
			if r.protocol != "tcp" && r.protocol != "udp" {
				return r, errInvalidRequest
			}
		} else {
			r.protocol = segs[1]
			if !symbolContainsWildcard(r.protocol) {
				return r, errInvalidRequest
			}
		}
		if r.port == "" || r.protocol == "" {
			return r, errInvalidRequest
		}
		offset = 2
	}
	if qtype == "A" && len(segs) == 4 {
		// This is an endpoint A record request. Get first element as endpoint.
		r.endpoint = segs[0]
		offset = 1
	}

	if len(segs) == (offset + 3) {
		r.service = segs[offset]
		r.namespace = segs[offset+1]
		r.typeName = segs[offset+2]

		return r, nil
	}

	if len(segs) == 1 && qtype == "TXT" {
		r.typeName = segs[0]
		return r, nil
	}

	return r, errInvalidRequest

}

// Records looks up services in kubernetes. If exact is true, it will lookup
// just this name. This is used when find matches when completing SRV lookups
// for instance.
func (k *Kubernetes) Records(r recordRequest) ([]msg.Service, error) {

	// Abort if the namespace does not contain a wildcard, and namespace is not published per CoreFile
	// Case where namespace contains a wildcard is handled in Get(...) method.
	if (!symbolContainsWildcard(r.namespace)) && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(r.namespace, k.Namespaces)) {
		return nil, errNsNotExposed
	}

	services, pods, err := k.get(r)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 && len(pods) == 0 {
		// Did not find item in k8s
		return nil, errNoItems
	}

	records := k.getRecordsForK8sItems(services, pods, r.zone)
	return records, nil
}

func endpointHostname(addr api.EndpointAddress) string {
	if addr.Hostname != "" {
		return strings.ToLower(addr.Hostname)
	}
	if strings.Contains(addr.IP, ".") {
		return strings.Replace(addr.IP, ".", "-", -1)
	}
	if strings.Contains(addr.IP, ":") {
		return strings.ToLower(strings.Replace(addr.IP, ":", "-", -1))
	}
	return ""
}

func (k *Kubernetes) getRecordsForK8sItems(services []service, pods []pod, zone string) []msg.Service {
	var records []msg.Service

	for _, svc := range services {

		key := svc.name + "." + svc.namespace + ".svc." + zone

		if svc.addr == api.ClusterIPNone {
			// This is a headless service, create records for each endpoint
			for _, ep := range svc.endpoints {
				ephostname := endpointHostname(ep.addr)
				s := msg.Service{
					Key:  msg.Path(strings.ToLower(ephostname+"."+key), "coredns"),
					Host: ep.addr.IP, Port: int(ep.port.Port),
				}
				records = append(records, s)

			}
		} else {
			// Create records for each exposed port...
			for _, p := range svc.ports {
				s := msg.Service{Key: msg.Path(strings.ToLower(key), "coredns"), Host: svc.addr, Port: int(p.Port)}
				records = append(records, s)
			}
		}
	}

	for _, p := range pods {
		key := p.name + "." + p.namespace + ".pod." + zone
		s := msg.Service{
			Key:  msg.Path(strings.ToLower(key), "coredns"),
			Host: p.addr,
		}
		records = append(records, s)
	}

	return records
}

func ipFromPodName(podname string) string {
	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		return strings.Replace(podname, "-", ".", -1)
	}
	return strings.Replace(podname, "-", ":", -1)
}

func (k *Kubernetes) findPods(namespace, podname string) (pods []pod, err error) {
	if k.PodMode == PodModeDisabled {
		return pods, errors.New("pod records disabled")
	}

	var ip string
	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		ip = strings.Replace(podname, "-", ".", -1)
	} else {
		ip = strings.Replace(podname, "-", ":", -1)
	}

	if k.PodMode == PodModeInsecure {
		s := pod{name: podname, namespace: namespace, addr: ip}
		pods = append(pods, s)
		return pods, nil
	}

	// PodModeVerified
	objList, err := k.APIConn.podLister.Indexer.ByIndex(podIPIndex, ip)
	if err != nil {
		return nil, err
	}

	nsWildcard := symbolContainsWildcard(namespace)
	for _, o := range objList {
		p, ok := o.(*api.Pod)
		if !ok {
			return nil, errors.New("expected type *api.Pod")
		}
		// If namespace has a wildcard, filter results against Corefile namespace list.
		if nsWildcard && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(p.Namespace, k.Namespaces)) {
			continue
		}
		// check for matching ip and namespace
		if ip == p.Status.PodIP && symbolMatches(namespace, p.Namespace, nsWildcard) {
			s := pod{name: podname, namespace: namespace, addr: ip}
			pods = append(pods, s)
			return pods, nil
		}
	}
	return pods, nil
}

// get retrieves matching data from the cache.
func (k *Kubernetes) get(r recordRequest) (services []service, pods []pod, err error) {
	switch {
	case r.typeName == "pod":
		pods, err = k.findPods(r.namespace, r.service)
		return nil, pods, err
	default:
		services, err = k.findServices(r)
		return services, nil, err
	}
}

func (k *Kubernetes) findServices(r recordRequest) ([]service, error) {
	serviceList := k.APIConn.ServiceList()

	var resultItems []service

	nsWildcard := symbolContainsWildcard(r.namespace)
	serviceWildcard := symbolContainsWildcard(r.service)
	portWildcard := symbolContainsWildcard(r.port) || r.port == ""
	protocolWildcard := symbolContainsWildcard(r.protocol) || r.protocol == ""

	for _, svc := range serviceList {
		if !(symbolMatches(r.namespace, svc.Namespace, nsWildcard) && symbolMatches(r.service, svc.Name, serviceWildcard)) {
			continue
		}
		// If namespace has a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if nsWildcard && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(svc.Namespace, k.Namespaces)) {
			continue
		}
		s := service{name: svc.Name, namespace: svc.Namespace, addr: svc.Spec.ClusterIP}
		if s.addr != api.ClusterIPNone {
			for _, p := range svc.Spec.Ports {
				if !(symbolMatches(r.port, strings.ToLower(p.Name), portWildcard) && symbolMatches(r.protocol, strings.ToLower(string(p.Protocol)), protocolWildcard)) {
					continue
				}
				s.ports = append(s.ports, p)
			}
			resultItems = append(resultItems, s)
			continue
		}
		// Headless service
		endpointsList, err := k.APIConn.epLister.List()
		if err != nil {
			continue
		}
		for _, ep := range endpointsList.Items {
			if ep.ObjectMeta.Name != svc.Name || ep.ObjectMeta.Namespace != svc.Namespace {
				continue
			}
			for _, eps := range ep.Subsets {
				for _, addr := range eps.Addresses {
					for _, p := range eps.Ports {
						ephostname := endpointHostname(addr)
						if r.endpoint != "" && r.endpoint != ephostname {
							continue
						}
						if !(symbolMatches(r.port, strings.ToLower(p.Name), portWildcard) && symbolMatches(r.protocol, strings.ToLower(string(p.Protocol)), protocolWildcard)) {
							continue
						}
						s.endpoints = append(s.endpoints, endpoint{addr: addr, port: p})
					}
				}
			}
		}
		resultItems = append(resultItems, s)
	}
	return resultItems, nil
}

func symbolMatches(queryString, candidateString string, wildcard bool) bool {
	if wildcard {
		return true
	}
	return queryString == candidateString
}

// getServiceRecordForIP: Gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) getServiceRecordForIP(ip, name string) []msg.Service {
	// First check services with cluster ips
	svcList, err := k.APIConn.svcLister.List(labels.Everything())
	if err != nil {
		return nil
	}
	for _, service := range svcList {
		if !dnsstrings.StringInSlice(service.Namespace, k.Namespaces) {
			continue
		}
		if service.Spec.ClusterIP == ip {
			domain := service.Name + "." + service.Namespace + ".svc." + k.PrimaryZone()
			return []msg.Service{{Host: domain}}
		}
	}
	// If no cluster ips match, search endpoints
	epList, err := k.APIConn.epLister.List()
	if err != nil {
		return nil
	}
	for _, ep := range epList.Items {
		if !dnsstrings.StringInSlice(ep.ObjectMeta.Namespace, k.Namespaces) {
			continue
		}
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if addr.IP == ip {
					domain := endpointHostname(addr) + "." + ep.ObjectMeta.Name + "." + ep.ObjectMeta.Namespace + ".svc." + k.PrimaryZone()
					return []msg.Service{{Host: domain}}
				}
			}
		}
	}
	return nil
}

// symbolContainsWildcard checks whether symbol contains a wildcard value
func symbolContainsWildcard(symbol string) bool {
	return (symbol == "*" || symbol == "any")
}
