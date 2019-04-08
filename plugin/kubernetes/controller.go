package kubernetes

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin/kubernetes/object"

	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	podIPIndex            = "PodIP"
	svcNameNamespaceIndex = "NameNamespace"
	svcIPIndex            = "ServiceIP"
	epNameNamespaceIndex  = "EndpointNameNamespace"
	epIPIndex             = "EndpointsIP"
)

type dnsController interface {
	ServiceList() []*object.Service
	EndpointsList() []*object.Endpoints
	SvcIndex(string) []*object.Service
	SvcIndexReverse(string) []*object.Service
	PodIndex(string) []*object.Pod
	EpIndex(string) []*object.Endpoints
	EpIndexReverse(string) []*object.Endpoints

	GetNodeByName(string) (*api.Node, error)
	GetNamespaceByName(string) (*api.Namespace, error)

	Run()
	HasSynced() bool
	Stop() error

	// Modified returns the timestamp of the most recent changes
	Modified() int64
}

type dnsControl struct {
	// Modified tracks timestamp of the most recent changes
	// It needs to be first because it is guaranteed to be 8-byte
	// aligned ( we use sync.LoadAtomic with this )
	modified int64

	client kubernetes.Interface

	selector          labels.Selector
	namespaceSelector labels.Selector

	svcController cache.Controller
	podController cache.Controller
	epController  cache.Controller
	nsController  cache.Controller

	svcLister cache.Indexer
	podLister cache.Indexer
	epLister  cache.Indexer
	nsLister  cache.Store

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}

	zones            []string
	endpointNameMode bool
}

type dnsControlOpts struct {
	initPodCache       bool
	initEndpointsCache bool
	resyncPeriod       time.Duration
	ignoreEmptyService bool

	// Label handling.
	labelSelector          *meta.LabelSelector
	selector               labels.Selector
	namespaceLabelSelector *meta.LabelSelector
	namespaceSelector      labels.Selector

	zones            []string
	endpointNameMode bool
}

// newDNSController creates a controller for CoreDNS.
func newdnsController(kubeClient kubernetes.Interface, opts dnsControlOpts) *dnsControl {
	dns := dnsControl{
		client:            kubeClient,
		selector:          opts.selector,
		namespaceSelector: opts.namespaceSelector,
		stopCh:            make(chan struct{}),
		zones:             opts.zones,
		endpointNameMode:  opts.endpointNameMode,
	}

	dns.svcLister, dns.svcController = object.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  serviceListFunc(dns.client, api.NamespaceAll, dns.selector),
			WatchFunc: serviceWatchFunc(dns.client, api.NamespaceAll, dns.selector),
		},
		&api.Service{},
		opts.resyncPeriod,
		cache.ResourceEventHandlerFuncs{AddFunc: dns.Add, UpdateFunc: dns.Update, DeleteFunc: dns.Delete},
		cache.Indexers{svcNameNamespaceIndex: svcNameNamespaceIndexFunc, svcIPIndex: svcIPIndexFunc},
		object.ToService,
	)

	if opts.initPodCache {
		dns.podLister, dns.podController = object.NewIndexerInformer(
			&cache.ListWatch{
				ListFunc:  podListFunc(dns.client, api.NamespaceAll, dns.selector),
				WatchFunc: podWatchFunc(dns.client, api.NamespaceAll, dns.selector),
			},
			&api.Pod{},
			opts.resyncPeriod,
			cache.ResourceEventHandlerFuncs{AddFunc: dns.Add, UpdateFunc: dns.Update, DeleteFunc: dns.Delete},
			cache.Indexers{podIPIndex: podIPIndexFunc},
			object.ToPod,
		)
	}

	if opts.initEndpointsCache {
		dns.epLister, dns.epController = object.NewIndexerInformer(
			&cache.ListWatch{
				ListFunc:  endpointsListFunc(dns.client, api.NamespaceAll, dns.selector),
				WatchFunc: endpointsWatchFunc(dns.client, api.NamespaceAll, dns.selector),
			},
			&api.Endpoints{},
			opts.resyncPeriod,
			cache.ResourceEventHandlerFuncs{AddFunc: dns.Add, UpdateFunc: dns.Update, DeleteFunc: dns.Delete},
			cache.Indexers{epNameNamespaceIndex: epNameNamespaceIndexFunc, epIPIndex: epIPIndexFunc},
			object.ToEndpoints)
	}

	dns.nsLister, dns.nsController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  namespaceListFunc(dns.client, dns.namespaceSelector),
			WatchFunc: namespaceWatchFunc(dns.client, dns.namespaceSelector),
		},
		&api.Namespace{},
		opts.resyncPeriod,
		cache.ResourceEventHandlerFuncs{})

	return &dns
}

func podIPIndexFunc(obj interface{}) ([]string, error) {
	p, ok := obj.(*object.Pod)
	if !ok {
		return nil, errObj
	}
	return []string{p.PodIP}, nil
}

func svcIPIndexFunc(obj interface{}) ([]string, error) {
	svc, ok := obj.(*object.Service)
	if !ok {
		return nil, errObj
	}
	if len(svc.ExternalIPs) == 0 {
		return []string{svc.ClusterIP}, nil
	}

	return append([]string{svc.ClusterIP}, svc.ExternalIPs...), nil
}

func svcNameNamespaceIndexFunc(obj interface{}) ([]string, error) {
	s, ok := obj.(*object.Service)
	if !ok {
		return nil, errObj
	}
	return []string{s.Index}, nil
}

func epNameNamespaceIndexFunc(obj interface{}) ([]string, error) {
	s, ok := obj.(*object.Endpoints)
	if !ok {
		return nil, errObj
	}
	return []string{s.Index}, nil
}

func epIPIndexFunc(obj interface{}) ([]string, error) {
	ep, ok := obj.(*object.Endpoints)
	if !ok {
		return nil, errObj
	}
	return ep.IndexIP, nil
}

func serviceListFunc(c kubernetes.Interface, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Services(ns).List(opts)
		return listV1, err
	}
}

func podListFunc(c kubernetes.Interface, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Pods(ns).List(opts)
		return listV1, err
	}
}

func endpointsListFunc(c kubernetes.Interface, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Endpoints(ns).List(opts)
		return listV1, err
	}
}

func namespaceListFunc(c kubernetes.Interface, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Namespaces().List(opts)
		return listV1, err
	}
}

// Stop stops the  controller.
func (dns *dnsControl) Stop() error {
	dns.stopLock.Lock()
	defer dns.stopLock.Unlock()

	// Only try draining the workqueue if we haven't already.
	if !dns.shutdown {
		close(dns.stopCh)
		dns.shutdown = true

		return nil
	}

	return fmt.Errorf("shutdown already in progress")
}

// Run starts the controller.
func (dns *dnsControl) Run() {
	go dns.svcController.Run(dns.stopCh)
	if dns.epController != nil {
		go dns.epController.Run(dns.stopCh)
	}
	if dns.podController != nil {
		go dns.podController.Run(dns.stopCh)
	}
	go dns.nsController.Run(dns.stopCh)
	<-dns.stopCh
}

// HasSynced calls on all controllers.
func (dns *dnsControl) HasSynced() bool {
	a := dns.svcController.HasSynced()
	b := true
	if dns.epController != nil {
		b = dns.epController.HasSynced()
	}
	c := true
	if dns.podController != nil {
		c = dns.podController.HasSynced()
	}
	d := dns.nsController.HasSynced()
	return a && b && c && d
}

func (dns *dnsControl) ServiceList() (svcs []*object.Service) {
	os := dns.svcLister.List()
	for _, o := range os {
		s, ok := o.(*object.Service)
		if !ok {
			continue
		}
		svcs = append(svcs, s)
	}
	return svcs
}

func (dns *dnsControl) EndpointsList() (eps []*object.Endpoints) {
	os := dns.epLister.List()
	for _, o := range os {
		ep, ok := o.(*object.Endpoints)
		if !ok {
			continue
		}
		eps = append(eps, ep)
	}
	return eps
}

func (dns *dnsControl) PodIndex(ip string) (pods []*object.Pod) {
	os, err := dns.podLister.ByIndex(podIPIndex, ip)
	if err != nil {
		return nil
	}
	for _, o := range os {
		p, ok := o.(*object.Pod)
		if !ok {
			continue
		}
		pods = append(pods, p)
	}
	return pods
}

func (dns *dnsControl) SvcIndex(idx string) (svcs []*object.Service) {
	os, err := dns.svcLister.ByIndex(svcNameNamespaceIndex, idx)
	if err != nil {
		return nil
	}
	for _, o := range os {
		s, ok := o.(*object.Service)
		if !ok {
			continue
		}
		svcs = append(svcs, s)
	}
	return svcs
}

func (dns *dnsControl) SvcIndexReverse(ip string) (svcs []*object.Service) {
	os, err := dns.svcLister.ByIndex(svcIPIndex, ip)
	if err != nil {
		return nil
	}

	for _, o := range os {
		s, ok := o.(*object.Service)
		if !ok {
			continue
		}
		svcs = append(svcs, s)
	}
	return svcs
}

func (dns *dnsControl) EpIndex(idx string) (ep []*object.Endpoints) {
	os, err := dns.epLister.ByIndex(epNameNamespaceIndex, idx)
	if err != nil {
		return nil
	}
	for _, o := range os {
		e, ok := o.(*object.Endpoints)
		if !ok {
			continue
		}
		ep = append(ep, e)
	}
	return ep
}

func (dns *dnsControl) EpIndexReverse(ip string) (ep []*object.Endpoints) {
	os, err := dns.epLister.ByIndex(epIPIndex, ip)
	if err != nil {
		return nil
	}
	for _, o := range os {
		e, ok := o.(*object.Endpoints)
		if !ok {
			continue
		}
		ep = append(ep, e)
	}
	return ep
}

// GetNodeByName return the node by name. If nothing is found an error is
// returned. This query causes a roundtrip to the k8s API server, so use
// sparingly. Currently this is only used for Federation.
func (dns *dnsControl) GetNodeByName(name string) (*api.Node, error) {
	v1node, err := dns.client.CoreV1().Nodes().Get(name, meta.GetOptions{})
	return v1node, err
}

// GetNamespaceByName returns the namespace by name. If nothing is found an error is returned.
func (dns *dnsControl) GetNamespaceByName(name string) (*api.Namespace, error) {
	os := dns.nsLister.List()
	for _, o := range os {
		ns, ok := o.(*api.Namespace)
		if !ok {
			continue
		}
		if name == ns.ObjectMeta.Name {
			return ns, nil
		}
	}
	return nil, fmt.Errorf("namespace not found")
}

func (dns *dnsControl) Add(obj interface{})               { dns.detectChanges(nil, obj) }
func (dns *dnsControl) Delete(obj interface{})            { dns.detectChanges(obj, nil) }
func (dns *dnsControl) Update(oldObj, newObj interface{}) { dns.detectChanges(oldObj, newObj) }

// detectChanges detects changes in objects, and updates the modified timestamp
func (dns *dnsControl) detectChanges(oldObj, newObj interface{}) {
	// If both objects have the same resource version, they are identical.
	if newObj != nil && oldObj != nil && (oldObj.(meta.Object).GetResourceVersion() == newObj.(meta.Object).GetResourceVersion()) {
		return
	}
	obj := newObj
	if obj == nil {
		obj = oldObj
	}
	switch ob := obj.(type) {
	case *object.Service:
		dns.updateModifed()
	case *object.Endpoints:
		if newObj == nil || oldObj == nil {
			dns.updateModifed()
			return
		}
		p := oldObj.(*object.Endpoints)
		// endpoint updates can come frequently, make sure it's a change we care about
		if endpointsEquivalent(p, ob) {
			return
		}
		dns.updateModifed()
	case *object.Pod:
		dns.updateModifed()
	default:
		log.Warningf("Updates for %T not supported.", ob)
	}
}

// subsetsEquivalent checks if two endpoint subsets are significantly equivalent
// I.e. that they have the same ready addresses, host names, ports (including protocol
// and service names for SRV)
func subsetsEquivalent(sa, sb object.EndpointSubset) bool {
	if len(sa.Addresses) != len(sb.Addresses) {
		return false
	}
	if len(sa.Ports) != len(sb.Ports) {
		return false
	}

	// in Addresses and Ports, we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	for addr, aaddr := range sa.Addresses {
		baddr := sb.Addresses[addr]
		if aaddr.IP != baddr.IP {
			return false
		}
		if aaddr.Hostname != baddr.Hostname {
			return false
		}
	}

	for port, aport := range sa.Ports {
		bport := sb.Ports[port]
		if aport.Name != bport.Name {
			return false
		}
		if aport.Port != bport.Port {
			return false
		}
		if aport.Protocol != bport.Protocol {
			return false
		}
	}
	return true
}

// endpointsEquivalent checks if the update to an endpoint is something
// that matters to us or if they are effectively equivalent.
func endpointsEquivalent(a, b *object.Endpoints) bool {

	if len(a.Subsets) != len(b.Subsets) {
		return false
	}

	// we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	for i, sa := range a.Subsets {
		sb := b.Subsets[i]
		if !subsetsEquivalent(sa, sb) {
			return false
		}
	}
	return true
}

func (dns *dnsControl) Modified() int64 {
	unix := atomic.LoadInt64(&dns.modified)
	return unix
}

// updateModified set dns.modified to the current time.
func (dns *dnsControl) updateModifed() {
	unix := time.Now().Unix()
	atomic.StoreInt64(&dns.modified, unix)
}

var errObj = errors.New("obj was not of the correct type")
