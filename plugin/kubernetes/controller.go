package kubernetes

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	dnswatch "github.com/coredns/coredns/plugin/pkg/watch"

	api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	podIPIndex            = "PodIP"
	svcNameNamespaceIndex = "NameNamespace"
	svcIPIndex            = "ServiceIP"
	epNameNamespaceIndex  = "EndpointNameNamespace"
	epIPIndex             = "EndpointsIP"
)

type dnsController interface {
	ServiceList() []*api.Service
	SvcIndex(string) []*api.Service
	SvcIndexReverse(string) []*api.Service
	PodIndex(string) []*api.Pod
	EpIndex(string) []*api.Endpoints
	EpIndexReverse(string) []*api.Endpoints
	EndpointsList() []*api.Endpoints

	GetNodeByName(string) (*api.Node, error)
	GetNamespaceByName(string) (*api.Namespace, error)

	Run()
	HasSynced() bool
	Stop() error

	// Modified returns the timestamp of the most recent changes
	Modified() int64

	// Watch-related items
	SetWatchChan(dnswatch.Chan)
	Watch(string) error
	StopWatching(string)
}

type dnsControl struct {
	// Modified tracks timestamp of the most recent changes
	// It needs to be first because it is guarnteed to be 8-byte
	// aligned ( we use sync.LoadAtomic with this )
	modified int64

	client *kubernetes.Clientset

	selector labels.Selector

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

	// watch-related items channel
	watchChan        dnswatch.Chan
	watched          map[string]bool
	zones            []string
	endpointNameMode bool
}

type dnsControlOpts struct {
	initPodCache       bool
	initEndpointsCache bool
	resyncPeriod       time.Duration
	ignoreEmptyService bool
	// Label handling.
	labelSelector *meta.LabelSelector
	selector      labels.Selector

	zones            []string
	endpointNameMode bool
}

// newDNSController creates a controller for CoreDNS.
func newdnsController(kubeClient *kubernetes.Clientset, opts dnsControlOpts) *dnsControl {
	dns := dnsControl{
		client:           kubeClient,
		selector:         opts.selector,
		stopCh:           make(chan struct{}),
		watched:          make(map[string]bool),
		zones:            opts.zones,
		endpointNameMode: opts.endpointNameMode,
	}

	dns.svcLister, dns.svcController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  serviceListFunc(dns.client, api.NamespaceAll, dns.selector),
			WatchFunc: serviceWatchFunc(dns.client, api.NamespaceAll, dns.selector),
		},
		&api.Service{},
		opts.resyncPeriod,
		cache.ResourceEventHandlerFuncs{AddFunc: dns.Add, UpdateFunc: dns.Update, DeleteFunc: dns.Delete},
		cache.Indexers{svcNameNamespaceIndex: svcNameNamespaceIndexFunc, svcIPIndex: svcIPIndexFunc})

	if opts.initPodCache {
		dns.podLister, dns.podController = cache.NewIndexerInformer(
			&cache.ListWatch{
				ListFunc:  podListFunc(dns.client, api.NamespaceAll, dns.selector),
				WatchFunc: podWatchFunc(dns.client, api.NamespaceAll, dns.selector),
			},
			&api.Pod{},
			opts.resyncPeriod,
			cache.ResourceEventHandlerFuncs{AddFunc: dns.Add, UpdateFunc: dns.Update, DeleteFunc: dns.Delete},
			cache.Indexers{podIPIndex: podIPIndexFunc})
	}

	if opts.initEndpointsCache {
		dns.epLister, dns.epController = cache.NewIndexerInformer(
			&cache.ListWatch{
				ListFunc:  endpointsListFunc(dns.client, api.NamespaceAll, dns.selector),
				WatchFunc: endpointsWatchFunc(dns.client, api.NamespaceAll, dns.selector),
			},
			&api.Endpoints{},
			opts.resyncPeriod,
			cache.ResourceEventHandlerFuncs{AddFunc: dns.Add, UpdateFunc: dns.Update, DeleteFunc: dns.Delete},
			cache.Indexers{epNameNamespaceIndex: epNameNamespaceIndexFunc, epIPIndex: epIPIndexFunc})
	}

	dns.nsLister, dns.nsController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  namespaceListFunc(dns.client, dns.selector),
			WatchFunc: namespaceWatchFunc(dns.client, dns.selector),
		},
		&api.Namespace{}, opts.resyncPeriod, cache.ResourceEventHandlerFuncs{})

	return &dns
}

func podIPIndexFunc(obj interface{}) ([]string, error) {
	p, ok := obj.(*api.Pod)
	if !ok {
		return nil, errors.New("obj was not an *api.Pod")
	}
	return []string{p.Status.PodIP}, nil
}

func svcIPIndexFunc(obj interface{}) ([]string, error) {
	svc, ok := obj.(*api.Service)
	if !ok {
		return nil, errors.New("obj was not an *api.Service")
	}
	return []string{svc.Spec.ClusterIP}, nil
}

func svcNameNamespaceIndexFunc(obj interface{}) ([]string, error) {
	s, ok := obj.(*api.Service)
	if !ok {
		return nil, errors.New("obj was not an *api.Service")
	}
	return []string{s.ObjectMeta.Name + "." + s.ObjectMeta.Namespace}, nil
}

func epNameNamespaceIndexFunc(obj interface{}) ([]string, error) {
	s, ok := obj.(*api.Endpoints)
	if !ok {
		return nil, errors.New("obj was not an *api.Endpoints")
	}
	return []string{s.ObjectMeta.Name + "." + s.ObjectMeta.Namespace}, nil
}

func epIPIndexFunc(obj interface{}) ([]string, error) {
	ep, ok := obj.(*api.Endpoints)
	if !ok {
		return nil, errors.New("obj was not an *api.Endpoints")
	}
	var idx []string
	for _, eps := range ep.Subsets {
		for _, addr := range eps.Addresses {
			idx = append(idx, addr.IP)
		}
	}
	return idx, nil
}

func serviceListFunc(c *kubernetes.Clientset, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Services(ns).List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func podListFunc(c *kubernetes.Clientset, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Pods(ns).List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func serviceWatchFunc(c *kubernetes.Clientset, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Services(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func podWatchFunc(c *kubernetes.Clientset, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Pods(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func endpointsListFunc(c *kubernetes.Clientset, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Endpoints(ns).List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func endpointsWatchFunc(c *kubernetes.Clientset, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Endpoints(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func namespaceListFunc(c *kubernetes.Clientset, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		listV1, err := c.CoreV1().Namespaces().List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func namespaceWatchFunc(c *kubernetes.Clientset, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		w, err := c.CoreV1().Namespaces().Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func (dns *dnsControl) SetWatchChan(c dnswatch.Chan) { dns.watchChan = c }
func (dns *dnsControl) StopWatching(qname string)    { delete(dns.watched, qname) }

func (dns *dnsControl) Watch(qname string) error {
	if dns.watchChan == nil {
		return fmt.Errorf("cannot start watch because the channel has not been set")
	}
	dns.watched[qname] = true
	return nil
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

func (dns *dnsControl) ServiceList() (svcs []*api.Service) {
	os := dns.svcLister.List()
	for _, o := range os {
		s, ok := o.(*api.Service)
		if !ok {
			continue
		}
		svcs = append(svcs, s)
	}
	return svcs
}

func (dns *dnsControl) PodIndex(ip string) (pods []*api.Pod) {
	if dns.podLister == nil {
		return nil
	}
	os, err := dns.podLister.ByIndex(podIPIndex, ip)
	if err != nil {
		return nil
	}
	for _, o := range os {
		p, ok := o.(*api.Pod)
		if !ok {
			continue
		}
		pods = append(pods, p)
	}
	return pods
}

func (dns *dnsControl) SvcIndex(idx string) (svcs []*api.Service) {
	if dns.svcLister == nil {
		return nil
	}
	os, err := dns.svcLister.ByIndex(svcNameNamespaceIndex, idx)
	if err != nil {
		return nil
	}
	for _, o := range os {
		s, ok := o.(*api.Service)
		if !ok {
			continue
		}
		svcs = append(svcs, s)
	}
	return svcs
}

func (dns *dnsControl) SvcIndexReverse(ip string) (svcs []*api.Service) {
	if dns.svcLister == nil {
		return nil
	}
	os, err := dns.svcLister.ByIndex(svcIPIndex, ip)
	if err != nil {
		return nil
	}

	for _, o := range os {
		s, ok := o.(*api.Service)
		if !ok {
			continue
		}
		svcs = append(svcs, s)
	}
	return svcs
}

func (dns *dnsControl) EpIndex(idx string) (ep []*api.Endpoints) {
	if dns.epLister == nil {
		return nil
	}
	os, err := dns.epLister.ByIndex(epNameNamespaceIndex, idx)
	if err != nil {
		return nil
	}
	for _, o := range os {
		e, ok := o.(*api.Endpoints)
		if !ok {
			continue
		}
		ep = append(ep, e)
	}
	return ep
}

func (dns *dnsControl) EpIndexReverse(ip string) (ep []*api.Endpoints) {
	if dns.svcLister == nil {
		return nil
	}
	os, err := dns.epLister.ByIndex(epIPIndex, ip)
	if err != nil {
		return nil
	}
	for _, o := range os {
		e, ok := o.(*api.Endpoints)
		if !ok {
			continue
		}
		ep = append(ep, e)
	}
	return ep
}

func (dns *dnsControl) EndpointsList() (eps []*api.Endpoints) {
	if dns.epLister == nil {
		return nil
	}
	os := dns.epLister.List()
	for _, o := range os {
		ep, ok := o.(*api.Endpoints)
		if !ok {
			continue
		}
		eps = append(eps, ep)
	}
	return eps
}

// GetNodeByName return the node by name. If nothing is found an error is
// returned. This query causes a roundtrip to the k8s API server, so use
// sparingly. Currently this is only used for Federation.
func (dns *dnsControl) GetNodeByName(name string) (*api.Node, error) {
	v1node, err := dns.client.CoreV1().Nodes().Get(name, meta.GetOptions{})
	if err != nil {
		return &api.Node{}, err
	}
	return v1node, nil
}

// GetNamespaceByName returns the namespace by name. If nothing is found an
// error is returned.
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

func (dns *dnsControl) Modified() int64 {
	unix := atomic.LoadInt64(&dns.modified)
	return unix
}

// updateModified set dns.modified to the current time.
func (dns *dnsControl) updateModifed() {
	unix := time.Now().Unix()
	atomic.StoreInt64(&dns.modified, unix)
}

func (dns *dnsControl) sendServiceUpdates(s *api.Service) {
	for i := range dns.zones {
		name := serviceFQDN(s, dns.zones[i])
		if _, ok := dns.watched[name]; ok {
			dns.watchChan <- name
		}
	}
}

func (dns *dnsControl) sendPodUpdates(p *api.Pod) {
	for i := range dns.zones {
		name := podFQDN(p, dns.zones[i])
		if _, ok := dns.watched[name]; ok {
			dns.watchChan <- name
		}
	}
}

func (dns *dnsControl) sendEndpointsUpdates(ep *api.Endpoints) {
	for _, zone := range dns.zones {
		names := append(endpointFQDN(ep, zone, dns.endpointNameMode), serviceFQDN(ep, zone))
		for _, name := range names {
			if _, ok := dns.watched[name]; ok {
				dns.watchChan <- name
			}
		}
	}
}

// endpointsSubsetDiffs returns an Endpoints struct containing the Subsets that have changed between a and b.
// When we notify clients of changed endpoints we only want to notify them of endpoints that have changed.
// The Endpoints API object holds more than one endpoint, held in a list of Subsets.  Each Subset refers to
// an endpoint.  So, here we create a new Endpoints struct, and populate it with only the endpoints that have changed.
// This new Endpoints object is later used to generate the list of endpoint FQDNs to send to the client.
// This function computes this literally by combining the sets (in a and not in b) union (in b and not in a).
func endpointsSubsetDiffs(a, b *api.Endpoints) *api.Endpoints {
	c := b.DeepCopy()
	c.Subsets = []api.EndpointSubset{}

	// In the following loop, the first iteration computes (in a but not in b).
	// The second iteration then adds (in b but not in a)
	// The end result is an Endpoints that only contains the subsets (endpoints) that are different between a and b.
	for _, abba := range [][]*api.Endpoints{{a, b}, {b, a}} {
		a := abba[0]
		b := abba[1]
	left:
		for _, as := range a.Subsets {
			for _, bs := range b.Subsets {
				if subsetsEquivalent(as, bs) {
					continue left
				}
			}
			c.Subsets = append(c.Subsets, as)
		}
	}
	return c
}

// sendUpdates sends a notification to the server if a watch
// is enabled for the qname
func (dns *dnsControl) sendUpdates(oldObj, newObj interface{}) {
	// If both objects have the same resource version, they are identical.
	if newObj != nil && oldObj != nil && (oldObj.(meta.Object).GetResourceVersion() == newObj.(meta.Object).GetResourceVersion()) {
		return
	}
	obj := newObj
	if obj == nil {
		obj = oldObj
	}
	switch ob := obj.(type) {
	case *api.Service:
		dns.updateModifed()
		dns.sendServiceUpdates(ob)
	case *api.Endpoints:
		if newObj == nil || oldObj == nil {
			dns.updateModifed()
			dns.sendEndpointsUpdates(ob)
			return
		}
		p := oldObj.(*api.Endpoints)
		// endpoint updates can come frequently, make sure it's a change we care about
		if endpointsEquivalent(p, ob) {
			return
		}
		dns.updateModifed()
		dns.sendEndpointsUpdates(endpointsSubsetDiffs(p, ob))
	case *api.Pod:
		dns.updateModifed()
		dns.sendPodUpdates(ob)
	default:
		log.Warningf("Updates for %T not supported.", ob)
	}
}

func (dns *dnsControl) Add(obj interface{})               { dns.sendUpdates(nil, obj) }
func (dns *dnsControl) Delete(obj interface{})            { dns.sendUpdates(obj, nil) }
func (dns *dnsControl) Update(oldObj, newObj interface{}) { dns.sendUpdates(oldObj, newObj) }

// subsetsEquivalent checks if two endpoint subsets are significantly equivalent
// I.e. that they have the same ready addresses, host names, ports (including protocol
// and service names for SRV)
func subsetsEquivalent(sa, sb api.EndpointSubset) bool {
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
func endpointsEquivalent(a, b *api.Endpoints) bool {

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
