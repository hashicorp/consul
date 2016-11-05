// Package etcd provides the etcd backend middleware.
package etcd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/pkg/singleflight"
	"github.com/miekg/coredns/middleware/proxy"
	"github.com/miekg/coredns/request"

	etcdc "github.com/coreos/etcd/client"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Etcd is a middleware talks to an etcd cluster.
type Etcd struct {
	Next       middleware.Handler
	Zones      []string
	PathPrefix string
	Proxy      proxy.Proxy // Proxy for looking up names during the resolution process
	Client     etcdc.KeysAPI
	Ctx        context.Context
	Inflight   *singleflight.Group
	Stubmap    *map[string]proxy.Proxy // list of proxies for stub resolving.
	Debugging  bool                    // Do we allow debug queries.

	endpoints []string // Stored here as well, to aid in testing.
}

// Services implements the ServiceBackend interface.
func (e *Etcd) Services(state request.Request, exact bool, opt middleware.Options) (services, debug []msg.Service, err error) {
	services, err = e.Records(state.Name(), exact)
	if err != nil {
		return
	}
	if opt.Debug != "" {
		debug = services
	}
	services = msg.Group(services)
	return
}

// Reverse implements the ServiceBackend interface.
func (e *Etcd) Reverse(state request.Request, exact bool, opt middleware.Options) (services, debug []msg.Service, err error) {
	return e.Services(state, exact, opt)
}

// Lookup implements the ServiceBackend interface.
func (e *Etcd) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return e.Proxy.Lookup(state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (e *Etcd) IsNameError(err error) bool {
	if ee, ok := err.(etcdc.Error); ok && ee.Code == etcdc.ErrorCodeKeyNotFound {
		return true
	}
	return false
}

// Debug implements the ServiceBackend interface.
func (e *Etcd) Debug() string {
	return e.PathPrefix
}

// Records looks up records in etcd. If exact is true, it will lookup just this
// name. This is used when find matches when completing SRV lookups for instance.
func (e *Etcd) Records(name string, exact bool) ([]msg.Service, error) {
	path, star := msg.PathWithWildcard(name, e.PathPrefix)
	r, err := e.get(path, true)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(msg.Path(name, e.PathPrefix), "/")
	switch {
	case exact && r.Node.Dir:
		return nil, nil
	case r.Node.Dir:
		return e.loopNodes(r.Node.Nodes, segments, star, nil)
	default:
		return e.loopNodes([]*etcdc.Node{r.Node}, segments, false, nil)
	}
}

// get is a wrapper for client.Get that uses SingleInflight to suppress multiple outstanding queries.
func (e *Etcd) get(path string, recursive bool) (*etcdc.Response, error) {
	resp, err := e.Inflight.Do(path, func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(e.Ctx, etcdTimeout)
		defer cancel()
		r, e := e.Client.Get(ctx, path, &etcdc.GetOptions{Sort: false, Recursive: recursive})
		if e != nil {
			return nil, e
		}
		return r, e
	})
	if err != nil {
		return nil, err
	}
	return resp.(*etcdc.Response), err
}

// skydns/local/skydns/east/staging/web
// skydns/local/skydns/west/production/web
//
// skydns/local/skydns/*/*/web
// skydns/local/skydns/*/web

// loopNodes recursively loops through the nodes and returns all the values. The nodes' keyname
// will be match against any wildcards when star is true.
func (e *Etcd) loopNodes(ns []*etcdc.Node, nameParts []string, star bool, bx map[msg.Service]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[msg.Service]bool)
	}
Nodes:
	for _, n := range ns {
		if n.Dir {
			nodes, err := e.loopNodes(n.Nodes, nameParts, star, bx)
			if err != nil {
				return nil, err
			}
			sx = append(sx, nodes...)
			continue
		}
		if star {
			keyParts := strings.Split(n.Key, "/")
			for i, n := range nameParts {
				if i > len(keyParts)-1 {
					// name is longer than key
					continue Nodes
				}
				if n == "*" || n == "any" {
					continue
				}
				if keyParts[i] != n {
					continue Nodes
				}
			}
		}
		serv := new(msg.Service)
		if err := json.Unmarshal([]byte(n.Value), serv); err != nil {
			return nil, fmt.Errorf("%s: %s", n.Key, err.Error())
		}
		b := msg.Service{Host: serv.Host, Port: serv.Port, Priority: serv.Priority, Weight: serv.Weight, Text: serv.Text, Key: n.Key}
		if _, ok := bx[b]; ok {
			continue
		}
		bx[b] = true

		serv.Key = n.Key
		serv.TTL = e.TTL(n, serv)
		if serv.Priority == 0 {
			serv.Priority = priority
		}
		sx = append(sx, *serv)
	}
	return sx, nil
}

// TTL returns the smaller of the etcd TTL and the service's
// TTL. If neither of these are set (have a zero value), a default is used.
func (e *Etcd) TTL(node *etcdc.Node, serv *msg.Service) uint32 {
	etcdTTL := uint32(node.TTL)

	if etcdTTL == 0 && serv.TTL == 0 {
		return ttl
	}
	if etcdTTL == 0 {
		return serv.TTL
	}
	if serv.TTL == 0 {
		return etcdTTL
	}
	if etcdTTL < serv.TTL {
		return etcdTTL
	}
	return serv.TTL
}

const (
	priority    = 10  // default priority when nothing is set
	ttl         = 300 // default ttl when nothing is set
	etcdTimeout = 5 * time.Second
)
