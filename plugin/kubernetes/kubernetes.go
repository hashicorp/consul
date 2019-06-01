// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
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
	Upstream         *upstream.Upstream
	APIServerList    []string
	APICertAuth      string
	APIClientCert    string
	APIClientKey     string
	ClientConfig     clientcmd.ClientConfig
	APIConn          dnsController
	Namespaces       map[string]struct{}
	podMode          string
	endpointNameMode bool
	Fall             fall.F
	ttl              uint32
	opts             dnsControlOpts

	primaryZoneIndex   int
	interfaceAddrsFunc func() net.IP
	autoPathSearch     []string // Local search path from /etc/resolv.conf. Needed for autopath.
	TransferTo         []string
}

// New returns a initialized Kubernetes. It default interfaceAddrFunc to return 127.0.0.1. All other
// values default to their zero value, primaryZoneIndex will thus point to the first zone.
func New(zones []string) *Kubernetes {
	k := new(Kubernetes)
	k.Zones = zones
	k.Namespaces = make(map[string]struct{})
	k.interfaceAddrsFunc = func() net.IP { return net.ParseIP("127.0.0.1") }
	k.podMode = podModeDisabled
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
func (k *Kubernetes) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (svcs []msg.Service, err error) {
	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {

	case dns.TypeTXT:
		// 1 label + zone, label must be "dns-version".
		t, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		segs := dns.SplitDomainName(t)
		if len(segs) != 1 {
			return nil, nil
		}
		if segs[0] != "dns-version" {
			return nil, nil
		}
		svc := msg.Service{Text: DNSSchemaVersion, TTL: 28800, Key: msg.Path(state.QName(), coredns)}
		return []msg.Service{svc}, nil

	case dns.TypeNS:
		// We can only get here if the qname equals the zone, see ServeDNS in handler.go.
		ns := k.nsAddr()
		svc := msg.Service{Host: ns.A.String(), Key: msg.Path(state.QName(), coredns), TTL: k.ttl}
		return []msg.Service{svc}, nil
	}

	if isDefaultNS(state.Name(), state.Zone) {
		ns := k.nsAddr()

		isIPv4 := ns.A.To4() != nil

		if !((state.QType() == dns.TypeA && isIPv4) || (state.QType() == dns.TypeAAAA && !isIPv4)) {
			// NODATA
			return nil, nil
		}

		// If this is an A request for "ns.dns", respond with a "fake" record for coredns.
		// SOA records always use this hardcoded name
		svc := msg.Service{Host: ns.A.String(), Key: msg.Path(state.QName(), coredns), TTL: k.ttl}
		return []msg.Service{svc}, nil
	}

	s, e := k.Records(ctx, state, false)

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
func (k *Kubernetes) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Upstream.Lookup(ctx, state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (k *Kubernetes) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest
}

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	if k.ClientConfig != nil {
		return k.ClientConfig.ClientConfig()
	}
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
	// Only the first one is used. We will deprecated multiple endpoints later.
	clusterinfo.Server = k.APIServerList[0]

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

// InitKubeCache initializes a new Kubernetes cache.
func (k *Kubernetes) InitKubeCache() (err error) {
	config, err := k.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes notification controller: %q", err)
	}

	if k.opts.labelSelector != nil {
		var selector labels.Selector
		selector, err = meta.LabelSelectorAsSelector(k.opts.labelSelector)
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", k.opts.labelSelector, err)
		}
		k.opts.selector = selector
	}

	if k.opts.namespaceLabelSelector != nil {
		var selector labels.Selector
		selector, err = meta.LabelSelectorAsSelector(k.opts.namespaceLabelSelector)
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", k.opts.namespaceLabelSelector, err)
		}
		k.opts.namespaceSelector = selector
	}

	k.opts.initPodCache = k.podMode == podModeVerified

	k.opts.zones = k.Zones
	k.opts.endpointNameMode = k.endpointNameMode
	k.APIConn = newdnsController(kubeClient, k.opts)

	return err
}

// Records looks up services in kubernetes.
func (k *Kubernetes) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	r, e := parseRequest(state)
	if e != nil {
		return nil, e
	}
	if r.podOrSvc == "" {
		return nil, nil
	}

	if dnsutil.IsReverse(state.Name()) > 0 {
		return nil, errNoItems
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

// serviceFQDN returns the k8s cluster dns spec service FQDN for the service (or endpoint) object.
func serviceFQDN(obj meta.Object, zone string) string {
	return dnsutil.Join(obj.GetName(), obj.GetNamespace(), Svc, zone)
}

// podFQDN returns the k8s cluster dns spec FQDN for the pod.
func podFQDN(p *object.Pod, zone string) string {
	if strings.Contains(p.PodIP, ".") {
		name := strings.Replace(p.PodIP, ".", "-", -1)
		return dnsutil.Join(name, p.GetNamespace(), Pod, zone)
	}

	name := strings.Replace(p.PodIP, ":", "-", -1)
	return dnsutil.Join(name, p.GetNamespace(), Pod, zone)
}

// endpointFQDN returns a list of k8s cluster dns spec service FQDNs for each subset in the endpoint.
func endpointFQDN(ep *object.Endpoints, zone string, endpointNameMode bool) []string {
	var names []string
	for _, ss := range ep.Subsets {
		for _, addr := range ss.Addresses {
			names = append(names, dnsutil.Join(endpointHostname(addr, endpointNameMode), serviceFQDN(ep, zone)))
		}
	}
	return names
}

func endpointHostname(addr object.EndpointAddress, endpointNameMode bool) string {
	if addr.Hostname != "" {
		return addr.Hostname
	}
	if endpointNameMode && addr.TargetRefName != "" {
		return addr.TargetRefName
	}
	if strings.Contains(addr.IP, ".") {
		return strings.Replace(addr.IP, ".", "-", -1)
	}
	if strings.Contains(addr.IP, ":") {
		return strings.Replace(addr.IP, ":", "-", -1)
	}
	return ""
}

func (k *Kubernetes) findPods(r recordRequest, zone string) (pods []msg.Service, err error) {
	if k.podMode == podModeDisabled {
		return nil, errNoItems
	}

	namespace := r.namespace
	if !wildcard(namespace) && !k.namespaceExposed(namespace) {
		return nil, errNoItems
	}

	podname := r.service

	// handle empty pod name
	if podname == "" {
		if k.namespaceExposed(namespace) || wildcard(namespace) {
			// NODATA
			return nil, nil
		}
		// NXDOMAIN
		return nil, errNoItems
	}

	zonePath := msg.Path(zone, coredns)
	ip := ""
	if strings.Count(podname, "-") == 3 && !strings.Contains(podname, "--") {
		ip = strings.Replace(podname, "-", ".", -1)
	} else {
		ip = strings.Replace(podname, "-", ":", -1)
	}

	if k.podMode == podModeInsecure {
		if !wildcard(namespace) && !k.namespaceExposed(namespace) { // no wildcard, but namespace does not exist
			return nil, errNoItems
		}

		// If ip does not parse as an IP address, we return an error, otherwise we assume a CNAME and will try to resolve it in backend_lookup.go
		if net.ParseIP(ip) == nil {
			return nil, errNoItems
		}

		return []msg.Service{{Key: strings.Join([]string{zonePath, Pod, namespace, podname}, "/"), Host: ip, TTL: k.ttl}}, err
	}

	// PodModeVerified
	err = errNoItems
	if wildcard(podname) && !wildcard(namespace) {
		// If namespace exists, err should be nil, so that we return NODATA instead of NXDOMAIN
		if k.namespaceExposed(namespace) {
			err = nil
		}
	}

	for _, p := range k.APIConn.PodIndex(ip) {
		// If namespace has a wildcard, filter results against Corefile namespace list.
		if wildcard(namespace) && !k.namespaceExposed(p.Namespace) {
			continue
		}

		// check for matching ip and namespace
		if ip == p.PodIP && match(namespace, p.Namespace) {
			s := msg.Service{Key: strings.Join([]string{zonePath, Pod, namespace, podname}, "/"), Host: ip, TTL: k.ttl}
			pods = append(pods, s)

			err = nil
		}
	}
	return pods, err
}

// findServices returns the services matching r from the cache.
func (k *Kubernetes) findServices(r recordRequest, zone string) (services []msg.Service, err error) {
	if !wildcard(r.namespace) && !k.namespaceExposed(r.namespace) {
		return nil, errNoItems
	}

	// handle empty service name
	if r.service == "" {
		if k.namespaceExposed(r.namespace) || wildcard(r.namespace) {
			// NODATA
			return nil, nil
		}
		// NXDOMAIN
		return nil, errNoItems
	}

	err = errNoItems
	if wildcard(r.service) && !wildcard(r.namespace) {
		// If namespace exists, err should be nil, so that we return NODATA instead of NXDOMAIN
		if k.namespaceExposed(r.namespace) {
			err = nil
		}
	}

	var (
		endpointsListFunc func() []*object.Endpoints
		endpointsList     []*object.Endpoints
		serviceList       []*object.Service
	)

	if wildcard(r.service) || wildcard(r.namespace) {
		serviceList = k.APIConn.ServiceList()
		endpointsListFunc = func() []*object.Endpoints { return k.APIConn.EndpointsList() }
	} else {
		idx := object.ServiceKey(r.service, r.namespace)
		serviceList = k.APIConn.SvcIndex(idx)
		endpointsListFunc = func() []*object.Endpoints { return k.APIConn.EpIndex(idx) }
	}

	zonePath := msg.Path(zone, coredns)
	for _, svc := range serviceList {
		if !(match(r.namespace, svc.Namespace) && match(r.service, svc.Name)) {
			continue
		}

		// If request namespace is a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if wildcard(r.namespace) && !k.namespaceExposed(svc.Namespace) {
			continue
		}

		// If "ignore empty_service" option is set and no endpoints exist, return NXDOMAIN unless
		// it's a headless or externalName service (covered below).
		if k.opts.ignoreEmptyService && svc.ClusterIP != api.ClusterIPNone && svc.Type != api.ServiceTypeExternalName {
			// serve NXDOMAIN if no endpoint is able to answer
			podsCount := 0
			for _, ep := range endpointsListFunc() {
				for _, eps := range ep.Subsets {
					podsCount = podsCount + len(eps.Addresses)
				}
			}

			if podsCount == 0 {
				continue
			}
		}

		// Endpoint query or headless service
		if svc.ClusterIP == api.ClusterIPNone || r.endpoint != "" {
			if endpointsList == nil {
				endpointsList = endpointsListFunc()
			}
			for _, ep := range endpointsList {
				if ep.Name != svc.Name || ep.Namespace != svc.Namespace {
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
		if svc.Type == api.ServiceTypeExternalName {
			s := msg.Service{Key: strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/"), Host: svc.ExternalName, TTL: k.ttl}
			if t, _ := s.HostType(); t == dns.TypeCNAME {
				s.Key = strings.Join([]string{zonePath, Svc, svc.Namespace, svc.Name}, "/")
				services = append(services, s)

				err = nil
			}
			continue
		}

		// ClusterIP service
		for _, p := range svc.Ports {
			if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
				continue
			}

			err = nil

			s := msg.Service{Host: svc.ClusterIP, Port: int(p.Port), TTL: k.ttl}
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

const coredns = "c" // used as a fake key prefix in msg.Service
