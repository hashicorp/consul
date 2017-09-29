package kubernetes

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
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

type dnsController interface {
	ServiceList() []*api.Service
	PodIndex(string) []*api.Pod
	EndpointsList() []*api.Endpoints

	GetNodeByName(string) (*api.Node, error)

	Run()
	Stop() error
}

type dnsControl struct {
	client *kubernetes.Clientset

	selector *labels.Selector

	svcController cache.Controller
	podController cache.Controller
	epController  cache.Controller

	svcLister cache.Indexer
	podLister cache.Indexer
	epLister  cache.Store

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
	selector      *labels.Selector
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
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

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
	dns.epLister, dns.epController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  endpointsListFunc(dns.client, namespace, dns.selector),
			WatchFunc: endpointsWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Endpoints{},
		opts.resyncPeriod,
		cache.ResourceEventHandlerFuncs{})

	return &dns
}

func podIPIndexFunc(obj interface{}) ([]string, error) {
	p, ok := obj.(*api.Pod)
	if !ok {
		return nil, errors.New("obj was not an *api.Pod")
	}
	return []string{p.Status.PodIP}, nil
}

func serviceListFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = (*s).String()
		}
		listV1, err := c.Services(ns).List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func podListFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = (*s).String()
		}
		listV1, err := c.Pods(ns).List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func serviceWatchFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = (*s).String()
		}
		w, err := c.Services(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func podWatchFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = (*s).String()
		}
		w, err := c.Pods(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func endpointsListFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = (*s).String()
		}
		listV1, err := c.Endpoints(ns).List(opts)
		if err != nil {
			return nil, err
		}
		return listV1, err
	}
}

func endpointsWatchFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = (*s).String()
		}
		w, err := c.Endpoints(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
}

func (dns *dnsControl) controllersInSync() bool {
	hs := dns.svcController.HasSynced() &&
		dns.epController.HasSynced()

	if dns.podController != nil {
		hs = hs && dns.podController.HasSynced()
	}

	return hs
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

func (dns *dnsControl) GetNodeByName(name string) (*api.Node, error) {
	v1node, err := dns.client.Nodes().Get(name, meta.GetOptions{})
	if err != nil {
		return &api.Node{}, err
	}
	return v1node, nil
}
