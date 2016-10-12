package kubernetes

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

var (
	namespace = api.NamespaceAll
)

// storeToNamespaceLister makes a Store that lists Namespaces.
type storeToNamespaceLister struct {
	cache.Store
}

// List lists all Namespaces in the store.
func (s *storeToNamespaceLister) List() (ns api.NamespaceList, err error) {
	for _, m := range s.Store.List() {
		ns.Items = append(ns.Items, *(m.(*api.Namespace)))
	}
	return ns, nil
}

type dnsController struct {
	client *client.Client

	selector *labels.Selector

	endpController *cache.Controller
	svcController  *cache.Controller
	nsController   *cache.Controller

	svcLister  cache.StoreToServiceLister
	endpLister cache.StoreToEndpointsLister
	nsLister   storeToNamespaceLister

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}
}

// newDNSController creates a controller for CoreDNS.
func newdnsController(kubeClient *client.Client, resyncPeriod time.Duration, lselector *labels.Selector) *dnsController {
	dns := dnsController{
		client:   kubeClient,
		selector: lselector,
		stopCh:   make(chan struct{}),
	}

	dns.endpLister.Store, dns.endpController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  endpointsListFunc(dns.client, namespace, dns.selector),
			WatchFunc: endpointsWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Endpoints{}, resyncPeriod, cache.ResourceEventHandlerFuncs{})

	dns.svcLister.Indexer, dns.svcController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  serviceListFunc(dns.client, namespace, dns.selector),
			WatchFunc: serviceWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Service{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	dns.nsLister.Store, dns.nsController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  namespaceListFunc(dns.client, dns.selector),
			WatchFunc: namespaceWatchFunc(dns.client, dns.selector),
		},
		&api.Namespace{}, resyncPeriod, cache.ResourceEventHandlerFuncs{})

	return &dns
}

func serviceListFunc(c *client.Client, ns string, s *labels.Selector) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = *s
		}
		return c.Services(ns).List(opts)
	}
}

func serviceWatchFunc(c *client.Client, ns string, s *labels.Selector) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = *s
		}
		return c.Services(ns).Watch(options)
	}
}

func endpointsListFunc(c *client.Client, ns string, s *labels.Selector) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = *s
		}
		return c.Endpoints(ns).List(opts)
	}
}

func endpointsWatchFunc(c *client.Client, ns string, s *labels.Selector) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = *s
		}
		return c.Endpoints(ns).Watch(options)
	}
}

func namespaceListFunc(c *client.Client, s *labels.Selector) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = *s
		}
		return c.Namespaces().List(opts)
	}
}

func namespaceWatchFunc(c *client.Client, s *labels.Selector) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = *s
		}
		return c.Namespaces().Watch(options)
	}
}

func (dns *dnsController) controllersInSync() bool {
	return dns.svcController.HasSynced() && dns.endpController.HasSynced()
}

// Stop stops the  controller.
func (dns *dnsController) Stop() error {
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
func (dns *dnsController) Run() {
	go dns.endpController.Run(dns.stopCh)
	go dns.svcController.Run(dns.stopCh)
	go dns.nsController.Run(dns.stopCh)
	<-dns.stopCh
}

func (dns *dnsController) NamespaceList() *api.NamespaceList {
	nsList, err := dns.nsLister.List()
	if err != nil {
		return &api.NamespaceList{}
	}

	return &nsList
}

func (dns *dnsController) ServiceList() []*api.Service {
	svcs, err := dns.svcLister.List(labels.Everything())
	if err != nil {
		return []*api.Service{}
	}

	return svcs
}

// ServicesByNamespace returns a map of:
//
// namespacename :: [ kubernetesService ]
func (dns *dnsController) ServicesByNamespace() map[string][]api.Service {
	k8sServiceList := dns.ServiceList()
	items := make(map[string][]api.Service, len(k8sServiceList))
	for _, i := range k8sServiceList {
		namespace := i.Namespace
		items[namespace] = append(items[namespace], *i)
	}

	return items
}

// ServiceInNamespace returns the Service that matches servicename in the namespace
func (dns *dnsController) ServiceInNamespace(namespace, servicename string) *api.Service {
	svcObj, err := dns.svcLister.Services(namespace).Get(servicename)
	if err != nil {
		// TODO(...): should return err here
		return nil
	}
	return svcObj
}
