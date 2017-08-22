// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/etcd/msg"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/middleware/pkg/healthcheck"
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
	Proxy         proxy.Proxy // Proxy for looking up names during the resolution process
	APIServerList []string
	APIProxy      *apiProxy
	APICertAuth   string
	APIClientCert string
	APIClientKey  string
	APIConn       dnsController
	Namespaces    map[string]bool
	podMode       string
	Fallthrough   bool

	primaryZoneIndex   int
	interfaceAddrsFunc func() net.IP
	autoPathSearch     []string // Local search path from /etc/resolv.conf. Needed for autopath.
}

// New returns a intialized Kubernetes. It default interfaceAddrFunc to return 127.0.0.1. All other
// values default to their zero value, primaryZoneIndex will thus point to the first zone.
func New(zones []string) *Kubernetes {
	k := new(Kubernetes)
	k.Zones = zones
	k.Namespaces = make(map[string]bool)
	k.interfaceAddrsFunc = func() net.IP { return net.ParseIP("127.0.0.1") }
	k.podMode = podModeDisabled
	k.Proxy = proxy.Proxy{}

	return k
}

const (
	// podModeDisabled is the default value where pod requests are ignored
	podModeDisabled = "disabled"
	// podModeVerified is where Pod requests are answered only if they exist
	podModeVerified = "verified"
	// podModeInsecure is where pod requests are answered without verfying they exist
	podModeInsecure = "insecure"
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
	errAPIBadPodType  = errors.New("expected type *api.Pod")
	errPodsDisabled   = errors.New("pod records disabled")
)

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(state request.Request, exact bool, opt middleware.Options) (svcs []msg.Service, debug []msg.Service, err error) {

	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {

	case dns.TypeTXT:
		// 1 label + zone, label must be "dns-version".
		t, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		segs := dns.SplitDomainName(t)
		if len(segs) != 1 {
			return nil, nil, fmt.Errorf("kubernetes: TXT query can only be for dns-version: %s", state.QName())
		}
		if segs[0] != "dns-version" {
			return nil, nil, nil
		}
		svc := msg.Service{Text: DNSSchemaVersion, TTL: 28800, Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil, nil

	case dns.TypeNS:
		// We can only get here if the qname equal the zone, see ServeDNS in handler.go.
		ns := k.nsAddr()
		svc := msg.Service{Host: ns.A.String(), Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil, nil
	}

	if state.QType() == dns.TypeA && isDefaultNS(state.Name(), state.Zone) {
		// If this is an A request for "ns.dns", respond with a "fake" record for coredns.
		// SOA records always use this hardcoded name
		ns := k.nsAddr()
		svc := msg.Service{Host: ns.A.String(), Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil, nil
	}

	s, e := k.Records(state, false)

	// SRV for external services is not yet implemented, so remove those records.

	if state.QType() != dns.TypeSRV {
		return s, nil, e
	}

	internal := []msg.Service{}
	for _, svc := range s {
		if t, _ := svc.HostType(); t != dns.TypeCNAME {
			internal = append(internal, svc)
		}
	}

	return internal, nil, e
}

// primaryZone will return the first non-reverse zone being handled by this middleware
func (k *Kubernetes) primaryZone() string {
	return k.Zones[k.primaryZoneIndex]
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
func (k *Kubernetes) Debug() string { return "debug" }

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}
	if len(k.APIServerList) > 0 {
		endpoint := k.APIServerList[0]
		if len(k.APIServerList) > 1 {
			// Use a random port for api proxy, will get the value later through listener.Addr()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return nil, fmt.Errorf("failed to create kubernetes api proxy: %v", err)
			}
			k.APIProxy = &apiProxy{
				listener: listener,
				handler: proxyHandler{
					HealthCheck: healthcheck.HealthCheck{
						FailTimeout: 3 * time.Second,
						MaxFails:    1,
						Future:      10 * time.Second,
						Path:        "/",
						Interval:    5 * time.Second,
					},
				},
			}
			k.APIProxy.handler.Hosts = make([]*healthcheck.UpstreamHost, len(k.APIServerList))
			for i, entry := range k.APIServerList {

				uh := &healthcheck.UpstreamHost{
					Name: strings.TrimPrefix(entry, "http://"),

					CheckDown: func(upstream *proxyHandler) healthcheck.UpstreamHostDownFunc {
						return func(uh *healthcheck.UpstreamHost) bool {

							down := false

							uh.CheckMu.Lock()
							until := uh.OkUntil
							uh.CheckMu.Unlock()

							if !until.IsZero() && time.Now().After(until) {
								down = true
							}

							fails := atomic.LoadInt32(&uh.Fails)
							if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
								down = true
							}
							return down
						}
					}(&k.APIProxy.handler),
				}

				k.APIProxy.handler.Hosts[i] = uh
			}
			k.APIProxy.Handler = &k.APIProxy.handler

			// Find the random port used for api proxy
			endpoint = fmt.Sprintf("http://%s", listener.Addr())
		}
		clusterinfo.Server = endpoint
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

// initKubeCache initializes a new Kubernetes cache.
func (k *Kubernetes) initKubeCache(opts dnsControlOpts) (err error) {

	config, err := k.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes notification controller: %q", err)
	}

	if opts.labelSelector != nil {
		var selector labels.Selector
		selector, err = unversionedapi.LabelSelectorAsSelector(opts.labelSelector)
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", opts.labelSelector, err)
		}
		opts.selector = &selector
	}

	opts.initPodCache = k.podMode == podModeVerified

	k.APIConn = newdnsController(kubeClient, opts)

	return err
}

// Records looks up services in kubernetes.
func (k *Kubernetes) Records(state request.Request, exact bool) ([]msg.Service, error) {
	r, e := k.parseRequest(state)
	if e != nil {
		return nil, e
	}

	if !wildcard(r.namespace) && !k.namespaceExposed(r.namespace) {
		return nil, errNsNotExposed
	}

	services, pods, err := k.get(r)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 && len(pods) == 0 {
		return nil, errNoItems
	}

	records := k.getRecordsForK8sItems(services, pods, state.Zone)
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

func (k *Kubernetes) getRecordsForK8sItems(services []kService, pods []kPod, zone string) (records []msg.Service) {
	zonePath := msg.Path(zone, "coredns")

	for _, svc := range services {
		if svc.addr == api.ClusterIPNone || len(svc.endpoints) > 0 {
			// This is a headless service or endpoints are present, create records for each endpoint
			for _, ep := range svc.endpoints {
				s := msg.Service{Host: ep.addr.IP, Port: int(ep.port.Port)}
				s.Key = strings.Join([]string{zonePath, Svc, svc.namespace, svc.name, endpointHostname(ep.addr)}, "/")

				records = append(records, s)
			}
			continue
		}

		// Create records for each exposed port...
		for _, p := range svc.ports {
			s := msg.Service{Host: svc.addr, Port: int(p.Port)}
			s.Key = strings.Join([]string{zonePath, Svc, svc.namespace, svc.name}, "/")

			records = append(records, s)
		}
		// If the addr is not an IP (i.e. an external service), add the record ...
		s := msg.Service{Key: strings.Join([]string{zonePath, Svc, svc.namespace, svc.name}, "/"), Host: svc.addr}
		if t, _ := s.HostType(); t == dns.TypeCNAME {
			s.Key = strings.Join([]string{zonePath, Svc, svc.namespace, svc.name}, "/")

			records = append(records, s)
		}
	}

	for _, p := range pods {
		s := msg.Service{Key: strings.Join([]string{zonePath, Pod, p.namespace, p.name}, "/"), Host: p.addr}
		records = append(records, s)
	}

	return records
}

func (k *Kubernetes) findPods(namespace, podname string) (pods []kPod, err error) {
	if k.podMode == podModeDisabled {
		return pods, errPodsDisabled
	}

	var ip string
	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		ip = strings.Replace(podname, "-", ".", -1)
	} else {
		ip = strings.Replace(podname, "-", ":", -1)
	}

	if k.podMode == podModeInsecure {
		s := kPod{name: podname, namespace: namespace, addr: ip}
		pods = append(pods, s)
		return pods, nil
	}

	// PodModeVerified
	objList := k.APIConn.PodIndex(ip)

	for _, o := range objList {
		p, ok := o.(*api.Pod)
		if !ok {
			return nil, errAPIBadPodType
		}
		// If namespace has a wildcard, filter results against Corefile namespace list.
		if wildcard(namespace) && !k.namespaceExposed(p.Namespace) {
			continue
		}
		// check for matching ip and namespace
		if ip == p.Status.PodIP && match(namespace, p.Namespace) {
			s := kPod{name: podname, namespace: namespace, addr: ip}
			pods = append(pods, s)
			return pods, nil
		}
	}
	return pods, nil
}

// get retrieves matching data from the cache.
func (k *Kubernetes) get(r recordRequest) (services []kService, pods []kPod, err error) {
	switch r.podOrSvc {
	case Pod:
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

	for _, svc := range serviceList {
		if !(match(r.namespace, svc.Namespace) && match(r.service, svc.Name)) {
			continue
		}

		// If namespace has a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if wildcard(r.namespace) && !k.namespaceExposed(svc.Namespace) {
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

						// See comments in parse.go parseRequest about the endpoint handling.

						if r.endpoint != "" {
							if !match(r.endpoint, endpointHostname(addr)) {
								continue
							}
						}

						for _, p := range eps.Ports {
							if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
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
			if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
				continue
			}
			s.ports = append(s.ports, p)
		}

		resultItems = append(resultItems, s)
	}
	return resultItems, nil
}

// match checks if a and b are equal taking wildcards into account.
func match(a, b string) bool {
	if wildcard(a) {
		return true
	}
	if wildcard(b) {
		return true
	}
	return strings.EqualFold(a, b)
}

// wildcard checks whether s contains a wildcard value defined as "*" or "any".
func wildcard(s string) bool {
	return s == "*" || s == "any"
}

// serviceRecordForIP gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) serviceRecordForIP(ip, name string) []msg.Service {
	// First check services with cluster ips
	svcList := k.APIConn.ServiceList()

	for _, service := range svcList {
		if (len(k.Namespaces) > 0) && !k.namespaceExposed(service.Namespace) {
			continue
		}
		if service.Spec.ClusterIP == ip {
			domain := dnsutil.Join([]string{service.Name, service.Namespace, Svc, k.primaryZone()})
			return []msg.Service{{Host: domain}}
		}
	}
	// If no cluster ips match, search endpoints
	epList := k.APIConn.EndpointsList()
	for _, ep := range epList.Items {
		if (len(k.Namespaces) > 0) && !k.namespaceExposed(ep.ObjectMeta.Namespace) {
			continue
		}
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if addr.IP == ip {
					domain := dnsutil.Join([]string{endpointHostname(addr), ep.ObjectMeta.Name, ep.ObjectMeta.Namespace, Svc, k.primaryZone()})
					return []msg.Service{{Host: domain}}
				}
			}
		}
	}
	return nil
}

// namespaceExposed returns true when the namespace is exposed.
func (k *Kubernetes) namespaceExposed(namespace string) bool {
	_, ok := k.Namespaces[namespace]
	if len(k.Namespaces) > 0 && !ok {
		return false
	}
	return true
}

const (
	// Svc is the DNS schema for kubernetes services
	Svc = "svc"
	// Pod is the DNS schema for kubernetes pods
	Pod = "pod"
)
