// Package etcd provides the etcd backend.
package etcd

import (
	"encoding/json"
	"strings"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/etcd/msg"
	"github.com/miekg/coredns/middleware/etcd/singleflight"
	"github.com/miekg/coredns/middleware/proxy"

	etcdc "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type Etcd struct {
	Next       middleware.Handler
	Zones      []string
	Proxy      proxy.Proxy // Proxy for looking up names during the resolution process
	Client     etcdc.KeysAPI
	Ctx        context.Context
	Inflight   *singleflight.Group
	PathPrefix string
}

func (g Etcd) Records(name string, exact bool) ([]msg.Service, error) {
	path, star := g.PathWithWildcard(name)
	r, err := g.Get(path, true)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(g.Path(name), "/")
	switch {
	case exact && r.Node.Dir:
		return nil, nil
	case r.Node.Dir:
		return g.loopNodes(r.Node.Nodes, segments, star, nil)
	default:
		return g.loopNodes([]*etcdc.Node{r.Node}, segments, false, nil)
	}
}

// Get is a wrapper for client.Get that uses SingleInflight to suppress multiple outstanding queries.
func (g Etcd) Get(path string, recursive bool) (*etcdc.Response, error) {
	resp, err := g.Inflight.Do(path, func() (interface{}, error) {
		r, e := g.Client.Get(g.Ctx, path, &etcdc.GetOptions{Sort: false, Recursive: recursive})
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
func (g Etcd) loopNodes(ns []*etcdc.Node, nameParts []string, star bool, bx map[msg.Service]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[msg.Service]bool)
	}
Nodes:
	for _, n := range ns {
		if n.Dir {
			nodes, err := g.loopNodes(n.Nodes, nameParts, star, bx)
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
			return nil, err
		}
		b := msg.Service{Host: serv.Host, Port: serv.Port, Priority: serv.Priority, Weight: serv.Weight, Text: serv.Text}
		if _, ok := bx[b]; ok {
			continue
		}
		bx[b] = true

		serv.Key = n.Key
		serv.Ttl = g.Ttl(n, serv)
		if serv.Priority == 0 {
			serv.Priority = priority
		}
		sx = append(sx, *serv)
	}
	return sx, nil
}

// Ttl returns the smaller of the etcd TTL and the service's
// TTL. If neither of these are set (have a zero value), a default is used.
func (g Etcd) Ttl(node *etcdc.Node, serv *msg.Service) uint32 {
	etcdTtl := uint32(node.TTL)

	if etcdTtl == 0 && serv.Ttl == 0 {
		return ttl
	}
	if etcdTtl == 0 {
		return serv.Ttl
	}
	if serv.Ttl == 0 {
		return etcdTtl
	}
	if etcdTtl < serv.Ttl {
		return etcdTtl
	}
	return serv.Ttl
}

// etcNameError checks if the error is ErrorCodeKeyNotFound from etcd.
func isEtcdNameError(err error) bool {
	if e, ok := err.(etcdc.Error); ok && e.Code == etcdc.ErrorCodeKeyNotFound {
		return true
	}
	return false
}

const (
	priority   = 10  // default priority when nothing is set
	ttl        = 300 // default ttl when nothing is set
	minTtl     = 60
	hostmaster = "hostmaster"
)
