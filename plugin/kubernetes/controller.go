package kubernetes

import (
	"errors"
	"fmt"
	"sync"
	"time"

	api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	namespace = api.NamespaceAll
)

const podIPIndex = "PodIP"
const svcNameNamespaceIndex = "NameNamespace"
const svcIPIndex = "ServiceIP"
const epNameNamespaceIndex = "EndpointNameNamespace"
const epIPIndex = "EndpointsIP"

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
}

type dnsControl struct {
	client *kubernetes.Clientset

	selector labels.Selector

	svcController cache.Controller
	podController cache.Controller
	epController  cache.Controller

	svcLister cache.Indexer
	podLister cache.Indexer
	epLister  cache.Indexer

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}
}

type dnsControlOpts struct {
	initPodCache bool
	resyncPeriod time.Duration
	// Label handling.
	labelSelector *meta.LabelSelector
	selector      labels.Selector
}

// newDNSController creates a controller for CoreDNS.
func newdnsController(kubeClient *kubernetes.Clientset, opts dnsControlOpts) *dnsControl {
	dns := dnsControl{
		client:   kubeClient,
		selector: opts.selector,
		stopCh:   make(chan struct{}),
	}
	dns.svcLister, dns.svcController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  serviceListFunc(dns.client, namespace, dns.selector),
			WatchFunc: serviceWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Service{},
		opts.resyncPeriod,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{svcNameNamespaceIndex: svcNameNamespaceIndexFunc, svcIPIndex: svcIPIndexFunc})

	if opts.initPodCache {
		dns.podLister, dns.podController = cache.NewIndexerInformer(
			&cache.ListWatch{
				ListFunc:  podListFunc(dns.client, namespace, dns.selector),
				WatchFunc: podWatchFunc(dns.client, namespace, dns.selector),
			},
			&api.Pod{}, // TODO replace with a lighter-weight custom struct
			opts.resyncPeriod,
			cache.ResourceEventHandlerFuncs{},
			cache.Indexers{podIPIndex: podIPIndexFunc})
	}
	dns.epLister, dns.epController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  endpointsListFunc(dns.client, namespace, dns.selector),
			WatchFunc: endpointsWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Endpoints{},
		opts.resyncPeriod,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{epNameNamespaceIndex: epNameNamespaceIndexFunc, epIPIndex: epIPIndexFunc})

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
	go dns.epController.Run(dns.stopCh)
	if dns.podController != nil {
		go dns.podController.Run(dns.stopCh)
	}
	<-dns.stopCh
}

// HasSynced calls on all controllers.
func (dns *dnsControl) HasSynced() bool {
	a := dns.svcController.HasSynced()
	b := dns.epController.HasSynced()
	c := true
	if dns.podController != nil {
		c = dns.podController.HasSynced()
	}
	return a && b && c
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
// error is returned. This query causes a roundtrip to the k8s API server, so
// use sparingly.
func (dns *dnsControl) GetNamespaceByName(name string) (*api.Namespace, error) {
	v1ns, err := dns.client.CoreV1().Namespaces().Get(name, meta.GetOptions{})
	if err != nil {
		return &api.Namespace{}, err
	}
	return v1ns, nil
}
