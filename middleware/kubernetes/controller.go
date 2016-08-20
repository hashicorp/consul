package kubernetes

import (
	"fmt"
	"sync"
	"time"

	"github.com/miekg/coredns/middleware/kubernetes/util"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

var (
	namespace = api.NamespaceAll
)

type dnsController struct {
	client *client.Client

	selector *labels.Selector

	endpController *framework.Controller
	svcController  *framework.Controller
	nsController   *framework.Controller

	svcLister  cache.StoreToServiceLister
	endpLister cache.StoreToEndpointsLister
	nsLister   util.StoreToNamespaceLister

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}
}

// newDNSController creates a controller for coredns
func newdnsController(kubeClient *client.Client, resyncPeriod time.Duration, lselector *labels.Selector) *dnsController {
	dns := dnsController{
		client:   kubeClient,
		selector: lselector,
		stopCh:   make(chan struct{}),
	}

	dns.endpLister.Store, dns.endpController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  endpointsListFunc(dns.client, namespace, dns.selector),
			WatchFunc: endpointsWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Endpoints{}, resyncPeriod, framework.ResourceEventHandlerFuncs{})

	dns.svcLister.Store, dns.svcController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  serviceListFunc(dns.client, namespace, dns.selector),
			WatchFunc: serviceWatchFunc(dns.client, namespace, dns.selector),
		},
		&api.Service{}, resyncPeriod, framework.ResourceEventHandlerFuncs{})

	dns.nsLister.Store, dns.nsController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc:  namespaceListFunc(dns.client, dns.selector),
			WatchFunc: namespaceWatchFunc(dns.client, dns.selector),
		},
		&api.Namespace{}, resyncPeriod, framework.ResourceEventHandlerFuncs{})

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

func (dns *dnsController) GetNamespaceList() *api.NamespaceList {
	nsList, err := dns.nsLister.List()
	if err != nil {
		return &api.NamespaceList{}
	}

	return &nsList
}

func (dns *dnsController) GetServiceList() *api.ServiceList {
	svcList, err := dns.svcLister.List()
	if err != nil {
		return &api.ServiceList{}
	}

	return &svcList
}

// GetServicesByNamespace returns a map of
// namespacename :: [ kubernetesService ]
func (dns *dnsController) GetServicesByNamespace() map[string][]api.Service {
	k8sServiceList := dns.GetServiceList()
	if k8sServiceList == nil {
		return nil
	}

	items := make(map[string][]api.Service, len(k8sServiceList.Items))
	for _, i := range k8sServiceList.Items {
		namespace := i.Namespace
		items[namespace] = append(items[namespace], i)
	}

	return items
}

// GetServiceInNamespace returns the Service that matches
// servicename in the namespace
func (dns *dnsController) GetServiceInNamespace(namespace string, servicename string) *api.Service {
	svcKey := fmt.Sprintf("%v/%v", namespace, servicename)
	svcObj, svcExists, err := dns.svcLister.Store.GetByKey(svcKey)

	if err != nil {
		// TODO(...): should return err here
		return nil
	}

	if !svcExists {
		// TODO(...): should return err here
		return nil
	}

	return svcObj.(*api.Service)
}
