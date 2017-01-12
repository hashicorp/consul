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
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"
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
	NameTemplate  *nametemplate.Template
	Namespaces    []string
	LabelSelector *unversionedapi.LabelSelector
	Selector      *labels.Selector
	PodMode       string
}

const (
	PodModeDisabled = "disabled" // default. pod requests are ignored
	PodModeInsecure = "insecure" // ALL pod requests are answered without verfying they exist
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

var errNoItems = errors.New("no items found")
var errNsNotExposed = errors.New("namespace is not exposed")
var errInvalidRequest = errors.New("invalid query name")

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(state request.Request, exact bool, opt middleware.Options) ([]msg.Service, []msg.Service, error) {
	if state.Type() == "SRV" && !ValidSRV(state.Name()) {
		return nil, nil, errInvalidRequest
	}
	s, e := k.Records(state.Name(), exact)
	return s, nil, e // Haven't implemented debug queries yet.
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

// stripSRVPrefix separates out the port and protocol segments, if present
// If not present, assume all ports/protocols (e.g. wildcard)
func stripSRVPrefix(name []string) (string, string, []string) {
	if name[0][0] == '_' && name[1][0] == '_' {
		return name[0][1:], name[1][1:], name[2:]
	}
	// no srv prefix present
	return "*", "*", name
}

func stripEndpointName(name []string) (endpoint string, nameOut []string) {
	if len(name) == 4 {
		return strings.ToLower(name[0]), name[1:]
	}
	return "", name
}

// Records looks up services in kubernetes. If exact is true, it will lookup
// just this name. This is used when find matches when completing SRV lookups
// for instance.
func (k *Kubernetes) Records(name string, exact bool) ([]msg.Service, error) {
	var (
		serviceName string
		namespace   string
		typeName    string
	)

	zone, serviceSegments := k.getZoneForName(name)
	port, protocol, serviceSegments := stripSRVPrefix(serviceSegments)
	endpointname, serviceSegments := stripEndpointName(serviceSegments)
	if len(serviceSegments) < 3 {
		return nil, errNoItems
	}

	serviceName = serviceSegments[0]
	namespace = serviceSegments[1]
	typeName = serviceSegments[2]

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

	// Abort if the namespace does not contain a wildcard, and namespace is not published per CoreFile
	// Case where namespace contains a wildcard is handled in Get(...) method.
	if (!symbolContainsWildcard(namespace)) && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(namespace, k.Namespaces)) {
		return nil, errNsNotExposed
	}

	services, pods, err := k.Get(namespace, serviceName, endpointname, port, protocol, typeName)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 && len(pods) == 0 {
		// Did not find item in k8s
		return nil, errNoItems
	}

	records := k.getRecordsForK8sItems(services, pods, zone)
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

// Get retrieves matching data from the cache.
func (k *Kubernetes) Get(namespace, servicename, endpointname, port, protocol, typeName string) (services []service, pods []pod, err error) {
	switch {
	case typeName == "pod":
		pods, err = k.findPods(namespace, servicename)
		return nil, pods, err
	default:
		services, err = k.findServices(namespace, servicename, endpointname, port, protocol)
		return services, nil, err
	}
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

	// TODO: implement cache verified pod responses
	return pods, nil

}

func (k *Kubernetes) findServices(namespace, servicename, endpointname, port, protocol string) ([]service, error) {
	serviceList := k.APIConn.ServiceList()

	var resultItems []service

	nsWildcard := symbolContainsWildcard(namespace)
	serviceWildcard := symbolContainsWildcard(servicename)
	portWildcard := symbolContainsWildcard(port)
	protocolWildcard := symbolContainsWildcard(protocol)

	for _, svc := range serviceList {
		if !(symbolMatches(namespace, svc.Namespace, nsWildcard) && symbolMatches(servicename, svc.Name, serviceWildcard)) {
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
				if !(symbolMatches(port, strings.ToLower(p.Name), portWildcard) && symbolMatches(protocol, strings.ToLower(string(p.Protocol)), protocolWildcard)) {
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
						if endpointname != "" && endpointname != ephostname {
							continue
						}
						if !(symbolMatches(port, strings.ToLower(p.Name), portWildcard) && symbolMatches(protocol, strings.ToLower(string(p.Protocol)), protocolWildcard)) {
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
			return []msg.Service{msg.Service{Host: domain}}
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
					return []msg.Service{msg.Service{Host: domain}}
				}
			}
		}
	}
	return nil
}

// symbolContainsWildcard checks whether symbol contains a wildcard value
func symbolContainsWildcard(symbol string) bool {
	return (strings.Contains(symbol, "*") || (symbol == "any"))
}

// ValidSRV parses a server record validating _port._proto. prefix labels.
// The valid schema is:
//   * Fist two segments must start with an "_",
//   * Second segment must be one of _tcp|_udp|_*|_any
func ValidSRV(name string) bool {

	// Does it start with a "_" ?
	if len(name) > 0 && name[0] != '_' {
		return false
	}

	// First label
	first, end := dns.NextLabel(name, 0)
	if end {
		return false
	}
	// Second label
	off, end := dns.NextLabel(name, first)
	if end {
		return false
	}

	// first:off has captured _tcp. or _udp. (if present)
	second := name[first:off]
	if len(second) > 0 && second[0] != '_' {
		return false
	}

	// A bit convoluted to avoid strings.ToLower
	if len(second) == 5 {
		// matches _tcp
		if (second[1] == 't' || second[1] == 'T') && (second[2] == 'c' || second[2] == 'C') &&
			(second[3] == 'p' || second[3] == 'P') {
			return true
		}
		// matches _udp
		if (second[1] == 'u' || second[1] == 'U') && (second[2] == 'd' || second[2] == 'D') &&
			(second[3] == 'p' || second[3] == 'P') {
			return true
		}
		// matches _any
		if (second[1] == 'a' || second[1] == 'A') && (second[2] == 'n' || second[2] == 'N') &&
			(second[3] == 'y' || second[3] == 'Y') {
			return true
		}
	}
	// matches _*
	if len(second) == 3 && second[1] == '*' {
		return true
	}

	return false
}
