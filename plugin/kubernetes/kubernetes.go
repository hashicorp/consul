// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Kubernetes implements a plugin that connects to a Kubernetes cluster.
type Kubernetes struct {
	Next             plugin.Handler
	Zones            []string
	Proxy            proxy.Proxy // Proxy for looking up names during the resolution process
	APIServerList    []string
	APIProxy         *apiProxy
	APICertAuth      string
	APIClientCert    string
	APIClientKey     string
	APIConn          dnsController
	Namespaces       map[string]bool
	podMode          string
	endpointNameMode bool
	Fall             fall.F
	ttl              uint32

	primaryZoneIndex   int
	interfaceAddrsFunc func() net.IP
	autoPathSearch     []string // Local search path from /etc/resolv.conf. Needed for autopath.
}

// New returns a initialized Kubernetes. It default interfaceAddrFunc to return 127.0.0.1. All other
// values default to their zero value, primaryZoneIndex will thus point to the first zone.
func New(zones []string) *Kubernetes {
	k := new(Kubernetes)
	k.Zones = zones
	k.Namespaces = make(map[string]bool)
	k.interfaceAddrsFunc = func() net.IP { return net.ParseIP("127.0.0.1") }
	k.podMode = podModeDisabled
	k.Proxy = proxy.Proxy{}
	k.ttl = defaultTTL

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
	// Svc is the DNS schema for kubernetes services
	Svc = "svc"
	// Pod is the DNS schema for kubernetes pods
	Pod = "pod"
	// defaultTTL to apply to all answers.
	defaultTTL = 5
)

var (
	errNoItems        = errors.New("no items found")
	errNsNotExposed   = errors.New("namespace is not exposed")
	errInvalidRequest = errors.New("invalid query name")
)

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(state request.Request, exact bool, opt plugin.Options) (svcs []msg.Service, err error) {

	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {

	case dns.TypeTXT:
		// 1 label + zone, label must be "dns-version".
		t, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		segs := dns.SplitDomainName(t)
		if len(segs) != 1 {
			return nil, fmt.Errorf("kubernetes: TXT query can only be for dns-version: %s", state.QName())
		}
		if segs[0] != "dns-version" {
			return nil, nil
		}
		svc := msg.Service{Text: DNSSchemaVersion, TTL: 28800, Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil

	case dns.TypeNS:
		// We can only get here if the qname equal the zone, see ServeDNS in handler.go.
		ns := k.nsAddr()
		svc := msg.Service{Host: ns.A.String(), Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil
	}

	if state.QType() == dns.TypeA && isDefaultNS(state.Name(), state.Zone) {
		// If this is an A request for "ns.dns", respond with a "fake" record for coredns.
		// SOA records always use this hardcoded name
		ns := k.nsAddr()
		svc := msg.Service{Host: ns.A.String(), Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil
	}

	s, e := k.Records(state, false)

	// SRV for external services is not yet implemented, so remove those records.

	if state.QType() != dns.TypeSRV {
		return s, e
	}

	internal := []msg.Service{}
	for _, svc := range s {
		if t, _ := svc.HostType(); t != dns.TypeCNAME {
			internal = append(internal, svc)
		}
	}

	return internal, e
}

// primaryZone will return the first non-reverse zone being handled by this plugin
func (k *Kubernetes) primaryZone() string { return k.Zones[k.primaryZoneIndex] }

// Lookup implements the ServiceBackend interface.
func (k *Kubernetes) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Proxy.Lookup(state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (k *Kubernetes) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest
}

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}

	// Connect to API from in cluster
	if len(k.APIServerList) == 0 {
		cc, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		cc.ContentType = "application/vnd.kubernetes.protobuf"
		return cc, err
	}

	// Connect to API from out of cluster
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

						fails := atomic.LoadInt32(&uh.Fails)
						if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
							return true
						}
						return false
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

	cc, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cc.ContentType = "application/vnd.kubernetes.protobuf"
	return cc, err

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
		selector, err = meta.LabelSelectorAsSelector(opts.labelSelector)
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", opts.labelSelector, err)
		}
		opts.selector = selector
	}

	opts.initPodCache = k.podMode == podModeVerified

	k.APIConn = newdnsController(kubeClient, opts)

	return err
}

// Records looks up services in kubernetes.
func (k *Kubernetes) Records(state request.Request, exact bool) ([]msg.Service, error) {
	r, e := parseRequest(state)
	if e != nil {
		return nil, e
	}

	if !wildcard(r.namespace) && !k.namespaceExposed(r.namespace) {
		return nil, errNsNotExposed
	}

	if r.podOrSvc == Pod {
		pods, err := k.findPods(r, state.Zone)
		return pods, err
	}

	services, err := k.findServices(r, state.Zone)
	return services, err
}

func endpointHostname(addr api.EndpointAddress, endpointNameMode bool) string {
	if addr.Hostname != "" {
		return strings.ToLower(addr.Hostname)
	}
	if endpointNameMode && addr.TargetRef != nil && addr.TargetRef.Name != "" {
		return addr.TargetRef.Name
	}
	if strings.Contains(addr.IP, ".") {
		return strings.Replace(addr.IP, ".", "-", -1)
	}
	if strings.Contains(addr.IP, ":") {
		return strings.ToLower(strings.Replace(addr.IP, ":", "-", -1))
	}
	return ""
}

func (k *Kubernetes) findPods(r recordRequest, zone string) (pods []msg.Service, err error) {
	if k.podMode == podModeDisabled {
		return nil, errNoItems
	}

	namespace := r.namespace
	podname := r.service
	zonePath := msg.Path(zone, "coredns")
	ip := ""

	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		ip = strings.Replace(podname, "-", ".", -1)
	} else {
		ip = strings.Replace(podname, "-", ":", -1)
	}

	if k.podMode == podModeInsecure {
		if !wildcard(namespace) && !k.namespace(namespace) { // no wildcard, but namespace does not exist
			return nil, errNoItems
		}

		// If ip does not parse as an IP address, we return an error, otherwise we assume a CNAME and will try to resolve it in backend_lookup.go
		if net.ParseIP(ip) == nil {
			return nil, errNoItems
		}

		return []msg.Service{{Key: strings.Join([]string{zonePath, Pod, namespace, podname}, "/"), Host: ip, TTL: k.ttl}}, err
	}

	err = errNoItems
	if wildcard(podname) && !wildcard(namespace) {
		// If namespace exist, err should be nil, so that we return nodata instead of NXDOMAIN
		if k.namespace(namespace) {
			err = nil
		}
	}

	// PodModeVerified
	for _, p := range k.APIConn.PodIndex(ip) {
		// If namespace has a wildcard, filter results against Corefile namespace list.
		if wildcard(namespace) && !k.namespaceExposed(p.Namespace) {
			continue
		}

		// check for matching ip and namespace
		if ip == p.Status.PodIP && match(namespace, p.Namespace) {
			s := msg.Service{Key: strings.Join([]string{zonePath, Pod, namespace, podname}, "/"), Host: ip, TTL: k.ttl}
			pods = append(pods, s)

			err = nil
		}
	}
	return pods, err
}

// findServices returns the services matching r from the cache.
func (k *Kubernetes) findServices(r recordRequest, zone string) (services []msg.Service, err error) {
	zonePath := msg.Path(zone, "coredns")

	err = errNoItems
	if wildcard(r.service) && !wildcard(r.namespace) {
		// If namespace exist, err should be nil, so that we return nodata instead of NXDOMAIN
		if k.namespace(namespace) {
			err = nil
		}
	}

	var (
		endpointsListFunc func() []*api.Endpoints
		endpointsList     []*api.Endpoints
		serviceList       []*api.Service
	)

	if wildcard(r.service) || wildcard(r.namespace) {
		serviceList = k.APIConn.ServiceList()
		endpointsListFunc = func() []*api.Endpoints { return k.APIConn.EndpointsList() }
	} else {
		idx := r.service + "." + r.namespace
		serviceList = k.APIConn.SvcIndex(idx)
		endpointsListFunc = func() []*api.Endpoints { return k.APIConn.EpIndex(idx) }
	}

	for _, svc := range serviceList {

		if !(match(r.namespace, svc.Namespace) && match(r.service, svc.Name)) {
			continue
		}

		// If namespace has a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if wildcard(r.namespace) && !k.namespaceExposed(svc.Namespace) {
			continue
		}

		// Endpoint query or headless service
		if svc.Spec.ClusterIP == api.ClusterIPNone || r.endpoint != "" {
			if endpointsList == nil {
				endpointsList = endpointsListFunc()
			}
			for _, ep := range endpointsList {
				if ep.ObjectMeta.Name != svc.Name || ep.ObjectMeta.Namespace != svc.Namespace {
					continue
				}

				for _, eps := range ep.Subsets {
					for _, addr := range eps.Addresses {

						// See comments in parse.go parseRequest about the endpoint handling.

						if r.endpoint != "" {
							if !match(r.endpoint, endpointHostname(addr, k.endpointNameMode)) {
								continue
							}
						}

						for _, p := range eps.Ports {
							if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
								continue
							}
							s := msg.Service{Host: addr.IP, Port: int(p.Port), TTL: k.ttl}
							s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name, endpointHostname(addr, k.endpointNameMode)}, "/")

							err = nil

							services = append(services, s)
						}
					}
				}
			}
			continue
		}

		// External service
		if svc.Spec.Type == api.ServiceTypeExternalName {
			s := msg.Service{Key: strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/"), Host: svc.Spec.ExternalName, TTL: k.ttl}
			if t, _ := s.HostType(); t == dns.TypeCNAME {
				s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")
				services = append(services, s)

				err = nil
			}
			continue
		}

		// ClusterIP service
		for _, p := range svc.Spec.Ports {
			if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
				continue
			}

			err = nil

			s := msg.Service{Host: svc.Spec.ClusterIP, Port: int(p.Port), TTL: k.ttl}
			s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")

			services = append(services, s)
		}
	}
	return services, err
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
