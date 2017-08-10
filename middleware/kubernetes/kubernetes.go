// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/etcd/msg"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	dnsstrings "github.com/coredns/coredns/middleware/pkg/strings"
	"github.com/coredns/coredns/middleware/proxy"
	"github.com/coredns/coredns/request"

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
	APIConn       dnsController
	ResyncPeriod  time.Duration
	Namespaces    []string
	Federations   []Federation
	LabelSelector *unversionedapi.LabelSelector
	Selector      *labels.Selector
	PodMode       string
	ReverseCidrs  []net.IPNet
	Fallthrough   bool

	interfaceAddrsFunc func() net.IP
}

const (
	// PodModeDisabled is the default value where pod requests are ignored
	PodModeDisabled = "disabled"
	// PodModeVerified is where Pod requests are answered only if they exist
	PodModeVerified = "verified"
	// PodModeInsecure is where pod requests are answered without verfying they exist
	PodModeInsecure = "insecure"
	// DNSSchemaVersion is the schema version: https://github.com/kubernetes/dns/blob/master/docs/specification.md
	DNSSchemaVersion = "1.0.1"
)

type endpoint struct {
	addr api.EndpointAddress
	port api.EndpointPort
}

// kService is a service as retrieved via the k8s API.
type kService struct {
	name      string
	namespace string
	addr      string
	ports     []api.ServicePort
	endpoints []endpoint
}

// kPod is a pod as retrieved via the k8s API.
type kPod struct {
	name      string
	namespace string
	addr      string
}

var (
	errNoItems        = errors.New("no items found")
	errNsNotExposed   = errors.New("namespace is not exposed")
	errInvalidRequest = errors.New("invalid query name")
	errZoneNotFound   = errors.New("zone not found")
	errAPIBadPodType  = errors.New("expected type *api.Pod")
	errPodsDisabled   = errors.New("pod records disabled")
)

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(state request.Request, exact bool, opt middleware.Options) (svcs []msg.Service, debug []msg.Service, err error) {

	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {
	case dns.TypeTXT:
		// 1 label + zone, label must be "dns-version"
		t, err := dnsutil.TrimZone(state.Name(), state.Zone)
		if err != nil {
			return nil, nil, err
		}
		segs := dns.SplitDomainName(t)
		if len(segs) != 1 {
			return nil, nil, errors.New("servfail")
		}
		if segs[0] != "dns-version" {
			return nil, nil, errInvalidRequest
		}
		svc := msg.Service{Text: DNSSchemaVersion, TTL: 28800, Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil, nil
	}

	r, e := k.parseRequest(state.Name(), state.QType(), state.Zone)
	if e != nil {
		return nil, nil, e
	}

	switch state.QType() {
	case dns.TypeA, dns.TypeAAAA, dns.TypeCNAME:
		if state.Type() == "A" && isDefaultNS(state.Name(), r) {
			// If this is an A request for "ns.dns", respond with a "fake" record for coredns.
			// SOA records always use this hardcoded name
			svcs = append(svcs, k.defaultNSMsg(r))
			return svcs, nil, nil
		}
		s, e := k.Entries(r)
		if state.QType() == dns.TypeAAAA {
			// AAAA not implemented
			return nil, nil, e
		}
		return s, nil, e // Haven't implemented debug queries yet.
	case dns.TypeSRV:
		s, e := k.Entries(r)
		// SRV for external services is not yet implemented, so remove those records
		noext := []msg.Service{}
		for _, svc := range s {
			if t, _ := svc.HostType(); t != dns.TypeCNAME {
				noext = append(noext, svc)
			}
		}
		return noext, nil, e
	case dns.TypeNS:
		srv := k.recordsForNS(r)
		svcs = append(svcs, srv)
		return svcs, nil, err
	}
	return nil, nil, nil
}

func (k *Kubernetes) recordsForNS(r recordRequest) msg.Service {
	ns := k.coreDNSRecord()
	return msg.Service{Host: ns.A.String(),
		Key: msg.Path(strings.Join([]string{ns.Hdr.Name, r.zone}, "."), "coredns")}
}

// PrimaryZone will return the first non-reverse zone being handled by this middleware
func (k *Kubernetes) PrimaryZone() string {
	return k.Zones[k.primaryZone]
}

// Lookup implements the ServiceBackend interface.
func (k *Kubernetes) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Proxy.Lookup(state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (k *Kubernetes) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest || err == errZoneNotFound
}

// Debug implements the ServiceBackend interface.
func (k *Kubernetes) Debug() string { return "debug" }

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
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
func (k *Kubernetes) InitKubeCache() (err error) {

	config, err := k.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes notification controller: %v", err)
	}

	if k.LabelSelector != nil {
		var selector labels.Selector
		selector, err = unversionedapi.LabelSelectorAsSelector(k.LabelSelector)
		k.Selector = &selector
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s'.Error was: %s", k.LabelSelector, err)
		}
	}

	if k.LabelSelector != nil {
		log.Printf("[INFO] Kubernetes middleware configured with the label selector '%s'. Only kubernetes objects matching this label selector will be exposed.", unversionedapi.FormatLabelSelector(k.LabelSelector))
	}

	opts := dnsControlOpts{
		initPodCache: k.PodMode == PodModeVerified,
	}
	k.APIConn = newdnsController(kubeClient, k.ResyncPeriod, k.Selector, opts)

	return err
}

// Records not implemented, see Entries().
func (k *Kubernetes) Records(name string, exact bool) ([]msg.Service, error) {
	return nil, fmt.Errorf("NOOP")
}

// Entries looks up services in kubernetes. If exact is true, it will lookup
// just this name. This is used when find matches when completing SRV lookups
// for instance.
func (k *Kubernetes) Entries(r recordRequest) ([]msg.Service, error) {

	// Abort if the namespace does not contain a wildcard, and namespace is not published per CoreFile
	// Case where namespace contains a wildcard is handled in Get(...) method.
	if (!wildcard(r.namespace)) && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(r.namespace, k.Namespaces)) {
		return nil, errNsNotExposed
	}
	services, pods, err := k.get(r)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 && len(pods) == 0 {
		// Did not find item in k8s, try federated
		if r.federation != "" {
			fedCNAME := k.federationCNAMERecord(r)
			if fedCNAME.Key != "" {
				return []msg.Service{fedCNAME}, nil
			}
		}
		return nil, errNoItems
	}

	records := k.getRecordsForK8sItems(services, pods, r)

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

func (k *Kubernetes) getRecordsForK8sItems(services []kService, pods []kPod, r recordRequest) (records []msg.Service) {
	zonePath := msg.Path(r.zone, "coredns")

	for _, svc := range services {
		if svc.addr == api.ClusterIPNone || len(svc.endpoints) > 0 {
			// This is a headless service or endpoints are present, create records for each endpoint
			for _, ep := range svc.endpoints {
				s := msg.Service{
					Host: ep.addr.IP,
					Port: int(ep.port.Port),
				}
				if r.federation != "" {
					s.Key = strings.Join([]string{zonePath, Svc, r.federation, svc.namespace, svc.name, endpointHostname(ep.addr)}, "/")
				} else {
					s.Key = strings.Join([]string{zonePath, Svc, svc.namespace, svc.name, endpointHostname(ep.addr)}, "/")
				}
				records = append(records, s)
			}
			continue
		}

		// Create records for each exposed port...
		for _, p := range svc.ports {
			s := msg.Service{
				Host: svc.addr,
				Port: int(p.Port)}

			if r.federation != "" {
				s.Key = strings.Join([]string{zonePath, Svc, r.federation, svc.namespace, svc.name}, "/")
			} else {
				s.Key = strings.Join([]string{zonePath, Svc, svc.namespace, svc.name}, "/")
			}

			records = append(records, s)
		}
		// If the addr is not an IP (i.e. an external service), add the record ...
		s := msg.Service{
			Key:  strings.Join([]string{zonePath, Svc, svc.namespace, svc.name}, "/"),
			Host: svc.addr}
		if t, _ := s.HostType(); t == dns.TypeCNAME {
			if r.federation != "" {
				s.Key = strings.Join([]string{zonePath, Svc, r.federation, svc.namespace, svc.name}, "/")
			} else {
				s.Key = strings.Join([]string{zonePath, Svc, svc.namespace, svc.name}, "/")
			}
			records = append(records, s)
		}
	}

	for _, p := range pods {
		s := msg.Service{
			Key:  strings.Join([]string{zonePath, Pod, p.namespace, p.name}, "/"),
			Host: p.addr,
		}
		records = append(records, s)
	}

	return records
}

func (k *Kubernetes) findPods(namespace, podname string) (pods []kPod, err error) {
	if k.PodMode == PodModeDisabled {
		return pods, errPodsDisabled
	}

	var ip string
	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		ip = strings.Replace(podname, "-", ".", -1)
	} else {
		ip = strings.Replace(podname, "-", ":", -1)
	}

	if k.PodMode == PodModeInsecure {
		s := kPod{name: podname, namespace: namespace, addr: ip}
		pods = append(pods, s)
		return pods, nil
	}

	// PodModeVerified
	objList := k.APIConn.PodIndex(ip)

	nsWildcard := wildcard(namespace)
	for _, o := range objList {
		p, ok := o.(*api.Pod)
		if !ok {
			return nil, errAPIBadPodType
		}
		// If namespace has a wildcard, filter results against Corefile namespace list.
		if nsWildcard && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(p.Namespace, k.Namespaces)) {
			continue
		}
		// check for matching ip and namespace
		if ip == p.Status.PodIP && match(namespace, p.Namespace, nsWildcard) {
			s := kPod{name: podname, namespace: namespace, addr: ip}
			pods = append(pods, s)
			return pods, nil
		}
	}
	return pods, nil
}

// get retrieves matching data from the cache.
func (k *Kubernetes) get(r recordRequest) (services []kService, pods []kPod, err error) {
	switch {
	case r.podOrSvc == Pod:
		pods, err = k.findPods(r.namespace, r.service)
		return nil, pods, err
	default:
		services, err = k.findServices(r)
		return services, nil, err
	}
}

func (k *Kubernetes) findServices(r recordRequest) ([]kService, error) {
	serviceList := k.APIConn.ServiceList()
	var resultItems []kService

	nsWildcard := wildcard(r.namespace)
	serviceWildcard := wildcard(r.service)
	portWildcard := wildcard(r.port) || r.port == ""
	protocolWildcard := wildcard(r.protocol) || r.protocol == ""

	for _, svc := range serviceList {
		if !(match(r.namespace, svc.Namespace, nsWildcard) && match(r.service, svc.Name, serviceWildcard)) {
			continue
		}
		// If namespace has a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if nsWildcard && (len(k.Namespaces) > 0) && (!dnsstrings.StringInSlice(svc.Namespace, k.Namespaces)) {
			continue
		}
		s := kService{name: svc.Name, namespace: svc.Namespace}

		// Endpoint query or headless service
		if svc.Spec.ClusterIP == api.ClusterIPNone || r.endpoint != "" {
			s.addr = svc.Spec.ClusterIP
			endpointsList := k.APIConn.EndpointsList()
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
							if !(match(r.port, p.Name, portWildcard) && match(r.protocol, string(p.Protocol), protocolWildcard)) {
								continue
							}
							s.endpoints = append(s.endpoints, endpoint{addr: addr, port: p})
						}
					}
				}
			}
			if len(s.endpoints) > 0 {
				resultItems = append(resultItems, s)
			}
			continue
		}

		// External service
		if svc.Spec.ExternalName != "" {
			s.addr = svc.Spec.ExternalName
			resultItems = append(resultItems, s)
			continue
		}

		// ClusterIP service
		s.addr = svc.Spec.ClusterIP
		for _, p := range svc.Spec.Ports {
			if !(match(r.port, p.Name, portWildcard) && match(r.protocol, string(p.Protocol), protocolWildcard)) {
				continue
			}
			s.ports = append(s.ports, p)
		}

		resultItems = append(resultItems, s)
	}
	return resultItems, nil
}

func match(a, b string, wildcard bool) bool {
	if wildcard {
		return true
	}
	return strings.EqualFold(a, b)
}

// getServiceRecordForIP: Gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) getServiceRecordForIP(ip, name string) []msg.Service {
	// First check services with cluster ips
	svcList := k.APIConn.ServiceList()

	for _, service := range svcList {
		if (len(k.Namespaces) > 0) && !dnsstrings.StringInSlice(service.Namespace, k.Namespaces) {
			continue
		}
		if service.Spec.ClusterIP == ip {
			domain := strings.Join([]string{service.Name, service.Namespace, Svc, k.PrimaryZone()}, ".")
			return []msg.Service{{Host: domain}}
		}
	}
	// If no cluster ips match, search endpoints
	epList := k.APIConn.EndpointsList()
	for _, ep := range epList.Items {
		if (len(k.Namespaces) > 0) && !dnsstrings.StringInSlice(ep.ObjectMeta.Namespace, k.Namespaces) {
			continue
		}
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if addr.IP == ip {
					domain := strings.Join([]string{endpointHostname(addr), ep.ObjectMeta.Name, ep.ObjectMeta.Namespace, Svc, k.PrimaryZone()}, ".")
					return []msg.Service{{Host: domain}}
				}
			}
		}
	}
	return nil
}

// wildcard checks whether s contains a wildcard value
func wildcard(s string) bool {
	return (s == "*" || s == "any")
}

func localPodIP() net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	for _, addr := range addrs {
		ip, _, _ := net.ParseCIDR(addr.String())
		ip = ip.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		return ip
	}
	return nil
}

const (
	// Svc is the DNS schema for kubernetes services
	Svc = "svc"
	// Pod is the DNS schema for kubernetes pods
	Pod = "pod"
)
