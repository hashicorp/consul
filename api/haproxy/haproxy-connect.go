package haproxy

import (
	"crypto/x509"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/connect/proxy"
)

const (
	defaultDownstreamBindAddr = "0.0.0.0"
	defaultUpstreamBindAddr   = "127.0.0.1"

	errorWaitTime = 5 * time.Second
)

type Config struct {
	ServiceName string
	ServiceID   string
	CAsPool     *x509.CertPool
	Downstream  Downstream
	Upstreams   []Upstream
}

type Upstream struct {
	Service          string
	LocalBindAddress string
	LocalBindPort    int

	TLS

	Nodes []UpstreamNode
}

func (n Upstream) Equal(o Upstream) bool {
	return n.LocalBindAddress == o.LocalBindAddress &&
		n.LocalBindPort == o.LocalBindPort &&
		n.TLS.Equal(o.TLS)
}

type UpstreamNode struct {
	Host   string
	Port   int
	Weight int
}

func (n UpstreamNode) ID() string {
	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}

func (n UpstreamNode) Equal(o UpstreamNode) bool {
	return n == o
}

type Downstream struct {
	LocalBindAddress string
	LocalBindPort    int
	TargetAddress    string
	TargetPort       int

	TLS
}

func (d Downstream) Equal(o Downstream) bool {
	return reflect.DeepEqual(d, o)
}

type TLS struct {
	Cert []byte
	Key  []byte
	CAs  [][]byte
}

func (t TLS) Equal(o TLS) bool {
	return reflect.DeepEqual(t, o)
}

type upstream struct {
	LocalBindAddress string
	LocalBindPort    int
	Service          string
	Datacenter       string
	Nodes            []*api.ServiceEntry

	done bool
}

type downstream struct {
	LocalBindAddress string
	LocalBindPort    int
	TargetAddress    string
	TargetPort       int
}

type certLeaf struct {
	Cert []byte
	Key  []byte

	done bool
}

type Watcher struct {
	service     string
	serviceName string
	consul      *api.Client
	token       string
	C           chan Config

	lock  sync.Mutex
	ready sync.WaitGroup

	upstreams  map[string]*upstream
	downstream downstream
	certCAs    [][]byte
	certCAPool *x509.CertPool
	leaf       *certLeaf

	update chan struct{}
}

func NewWatcher(service string, consul *api.Client) *Watcher {
	return &Watcher{
		service: service,
		consul:  consul,

		C:         make(chan Config),
		upstreams: make(map[string]*upstream),
		update:    make(chan struct{}, 1),
	}
}

func (w *Watcher) Run() error {
	proxyID, err := proxy.LookupProxyIDForSidecar(w.consul, w.service)
	if err != nil {
		return err
	}

	svc, _, err := w.consul.Agent().Service(w.service, &api.QueryOptions{})
	if err != nil {
		return err
	}

	w.serviceName = svc.Service

	w.ready.Add(4)

	go w.watchCA()
	go w.watchLeaf()
	go w.watchService(proxyID, w.handleProxyChange)
	go w.watchService(w.service, func(first bool, srv *api.AgentService) {
		w.downstream.TargetPort = srv.Port
		if first {
			w.ready.Done()
		}
	})

	w.ready.Wait()

	for range w.update {
		w.C <- w.genCfg()
	}

	return nil
}

func (w *Watcher) handleProxyChange(first bool, srv *api.AgentService) {
	w.downstream.LocalBindAddress = defaultDownstreamBindAddr
	w.downstream.LocalBindPort = srv.Port
	w.downstream.TargetAddress = defaultUpstreamBindAddr
	if srv.Connect != nil && srv.Connect.SidecarService != nil && srv.Connect.SidecarService.Proxy != nil && srv.Connect.SidecarService.Proxy.Config != nil {
		if b, ok := srv.Connect.SidecarService.Proxy.Config["bind_address"].(string); ok {
			w.downstream.LocalBindAddress = b
		}
		if a, ok := srv.Connect.SidecarService.Proxy.Config["local_service_address"].(string); ok {
			w.downstream.TargetAddress = a
		}
	}

	keep := make(map[string]bool)

	if srv.Proxy != nil {
		for _, up := range srv.Proxy.Upstreams {
			keep[up.DestinationName] = true
			w.lock.Lock()
			_, ok := w.upstreams[up.DestinationName]
			w.lock.Unlock()
			if !ok {
				w.startUpstream(up)
			}
		}
	}

	for name := range w.upstreams {
		if !keep[name] {
			w.removeUpstream(name)
		}
	}

	if first {
		w.ready.Done()
	}
}

func (w *Watcher) startUpstream(up api.Upstream) {

	u := &upstream{
		LocalBindAddress: up.LocalBindAddress,
		LocalBindPort:    up.LocalBindPort,
		Service:          up.DestinationName,
		Datacenter:       up.Datacenter,
	}

	w.lock.Lock()
	w.upstreams[up.DestinationName] = u
	w.lock.Unlock()

	go func() {
		index := uint64(0)
		for {
			if u.done {
				return
			}
			nodes, meta, err := w.consul.Health().Connect(up.DestinationName, "", true, &api.QueryOptions{
				Datacenter: up.Datacenter,
				WaitTime:   10 * time.Minute,
				WaitIndex:  index,
			})
			if err != nil {
				time.Sleep(errorWaitTime)
				index = 0
				continue
			}
			changed := index != meta.LastIndex
			index = meta.LastIndex

			if changed {
				w.lock.Lock()
				u.Nodes = nodes
				w.lock.Unlock()
				w.notifyChanged()
			}
		}
	}()
}

func (w *Watcher) removeUpstream(name string) {

	w.lock.Lock()
	w.upstreams[name].done = true
	delete(w.upstreams, name)
	w.lock.Unlock()
}

func (w *Watcher) watchLeaf() {
	var lastIndex uint64
	first := true
	for {
		cert, meta, err := w.consul.Agent().ConnectCALeaf(w.serviceName, &api.QueryOptions{
			WaitTime:  10 * time.Minute,
			WaitIndex: lastIndex,
		})
		if err != nil {
			time.Sleep(errorWaitTime)
			lastIndex = 0
			continue
		}

		changed := lastIndex != meta.LastIndex
		lastIndex = meta.LastIndex

		if changed {
			w.lock.Lock()
			if w.leaf == nil {
				w.leaf = &certLeaf{}
			}
			w.leaf.Cert = []byte(cert.CertPEM)
			w.leaf.Key = []byte(cert.PrivateKeyPEM)
			w.lock.Unlock()
			w.notifyChanged()
		}

		if first {
			w.ready.Done()
			first = false
		}
	}
}

func (w *Watcher) watchService(service string, handler func(first bool, srv *api.AgentService)) {

	hash := ""
	first := true
	for {
		srv, meta, err := w.consul.Agent().Service(service, &api.QueryOptions{
			WaitHash: hash,
			WaitTime: 10 * time.Minute,
		})
		if err != nil {
			time.Sleep(errorWaitTime)
			hash = ""
			continue
		}

		changed := hash != meta.LastContentHash
		hash = meta.LastContentHash

		if changed {
			handler(first, srv)
			w.notifyChanged()
		}

		first = false
	}
}

func (w *Watcher) watchCA() {

	first := true
	var lastIndex uint64
	for {
		caList, meta, err := w.consul.Agent().ConnectCARoots(&api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  10 * time.Minute,
		})
		if err != nil {
			time.Sleep(errorWaitTime)
			lastIndex = 0
			continue
		}

		changed := lastIndex != meta.LastIndex
		lastIndex = meta.LastIndex

		if changed {
			w.lock.Lock()
			w.certCAs = w.certCAs[:0]
			w.certCAPool = x509.NewCertPool()
			for _, ca := range caList.Roots {
				w.certCAs = append(w.certCAs, []byte(ca.RootCertPEM))
				ok := w.certCAPool.AppendCertsFromPEM([]byte(ca.RootCertPEM))
				if !ok {
					fmt.Println("FATAL: CONSUL: unable to add CA certificate to pool")
				}
			}
			w.lock.Unlock()
			w.notifyChanged()
		}

		if first {
			w.ready.Done()
			first = false
		}
	}
}

func (w *Watcher) genCfg() Config {
	w.lock.Lock()
	serviceInstancesAlive := 0
	serviceInstancesTotal := 0
	defer func() {
		w.lock.Unlock()
	}()

	config := Config{
		ServiceName: w.serviceName,
		ServiceID:   w.service,
		CAsPool:     w.certCAPool,
		Downstream: Downstream{
			LocalBindAddress: w.downstream.LocalBindAddress,
			LocalBindPort:    w.downstream.LocalBindPort,
			TargetAddress:    w.downstream.TargetAddress,
			TargetPort:       w.downstream.TargetPort,

			TLS: TLS{
				CAs:  w.certCAs,
				Cert: w.leaf.Cert,
				Key:  w.leaf.Key,
			},
		},
	}

	for _, up := range w.upstreams {
		upstream := Upstream{
			Service:          up.Service,
			LocalBindAddress: up.LocalBindAddress,
			LocalBindPort:    up.LocalBindPort,

			TLS: TLS{
				CAs:  w.certCAs,
				Cert: w.leaf.Cert,
				Key:  w.leaf.Key,
			},
		}

		for _, s := range up.Nodes {
			serviceInstancesTotal++
			host := s.Service.Address
			if host == "" {
				host = s.Node.Address
			}

			weight := 1
			switch s.Checks.AggregatedStatus() {
			case api.HealthPassing:
				weight = s.Service.Weights.Passing
			case api.HealthWarning:
				weight = s.Service.Weights.Warning
			default:
				continue
			}
			if weight == 0 {
				continue
			}
			serviceInstancesAlive++

			upstream.Nodes = append(upstream.Nodes, UpstreamNode{
				Host:   host,
				Port:   s.Service.Port,
				Weight: weight,
			})
		}

		config.Upstreams = append(config.Upstreams, upstream)
	}

	return config
}

func (w *Watcher) notifyChanged() {
	select {
	case w.update <- struct{}{}:
	default:
	}
}
