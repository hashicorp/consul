package kubernetes

import (
	"fmt"
	"log"
	"sync"
	"time"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/labels"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/watch"
	"k8s.io/client-go/1.5/tools/cache"
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
	client *kubernetes.Clientset

	selector *labels.Selector

	svcController *cache.Controller
	nsController  *cache.Controller

	svcLister cache.StoreToServiceLister
	nsLister  storeToNamespaceLister

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}
}

// newDNSController creates a controller for CoreDNS.
func newdnsController(kubeClient *kubernetes.Clientset, resyncPeriod time.Duration, lselector *labels.Selector) *dnsController {
	dns := dnsController{
		client:   kubeClient,
		selector: lselector,
		stopCh:   make(chan struct{}),
	}

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

func serviceListFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = *s
		}
		listV1, err := c.Core().Services(ns).List(opts)
		if err != nil {
			return nil, err
		}
		var listAPI api.ServiceList
		err = v1.Convert_v1_ServiceList_To_api_ServiceList(listV1, &listAPI, nil)
		if err != nil {
			return nil, err
		}
		return &listAPI, err
	}
}

func v1ToAPIFilter(in watch.Event) (out watch.Event, keep bool) {
	if in.Type == watch.Error {
		return in, true
	}

	switch v1Obj := in.Object.(type) {
	case *v1.Service:
		var apiObj api.Service
		err := v1.Convert_v1_Service_To_api_Service(v1Obj, &apiObj, nil)
		if err != nil {
			log.Printf("[ERROR] Could not convert v1.Service: %s", err)
			return in, true
		}
		return watch.Event{Type: in.Type, Object: &apiObj}, true
	case *v1.Namespace:
		var apiObj api.Namespace
		err := v1.Convert_v1_Namespace_To_api_Namespace(v1Obj, &apiObj, nil)
		if err != nil {
			log.Printf("[ERROR] Could not convert v1.Namespace: %s", err)
			return in, true
		}
		return watch.Event{Type: in.Type, Object: &apiObj}, true
	}

	log.Printf("[WARN] Unhandled v1 type in event: %v", in)
	return in, true
}

func serviceWatchFunc(c *kubernetes.Clientset, ns string, s *labels.Selector) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = *s
		}
		w, err := c.Core().Services(ns).Watch(options)
		if err != nil {
			return nil, err
		}
		return watch.Filter(w, v1ToAPIFilter), nil
	}
}

func namespaceListFunc(c *kubernetes.Clientset, s *labels.Selector) func(api.ListOptions) (runtime.Object, error) {
	return func(opts api.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = *s
		}
		listV1, err := c.Core().Namespaces().List(opts)
		if err != nil {
			return nil, err
		}
		var listAPI api.NamespaceList
		err = v1.Convert_v1_NamespaceList_To_api_NamespaceList(listV1, &listAPI, nil)
		if err != nil {
			return nil, err
		}
		return &listAPI, err
	}
}

func namespaceWatchFunc(c *kubernetes.Clientset, s *labels.Selector) func(options api.ListOptions) (watch.Interface, error) {
	return func(options api.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = *s
		}
		w, err := c.Core().Namespaces().Watch(options)
		if err != nil {
			return nil, err
		}
		return watch.Filter(w, v1ToAPIFilter), nil
	}
}

func (dns *dnsController) controllersInSync() bool {
	return dns.svcController.HasSynced()
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
