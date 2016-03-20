// Package etcd provides the etcd server Backend implementation,
package etcd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/miekg/coredns/middleware/etcd/msg"

	etcdc "github.com/coreos/etcd/client"
)

func (g *Backend) Records(name string, exact bool) ([]msg.Service, error) {
	path, star := msg.PathWithWildcard(name)
	r, err := g.get(path, true)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(msg.Path(name), "/")
	switch {
	case exact && r.Node.Dir:
		return nil, nil
	case r.Node.Dir:
		return g.loopNodes(r.Node.Nodes, segments, star, nil)
	default:
		return g.loopNodes([]*etcdc.Node{r.Node}, segments, false, nil)
	}
}

func (g *Backend) ReverseRecord(name string) (*msg.Service, error) {
	path, star := msg.PathWithWildcard(name)
	if star {
		return nil, fmt.Errorf("reverse can not contain wildcards")
	}
	r, err := g.get(path, true)
	if err != nil {
		return nil, err
	}
	if r.Node.Dir {
		return nil, fmt.Errorf("reverse must not be a directory")
	}
	segments := strings.Split(msg.Path(name), "/")
	records, err := g.loopNodes([]*etcdc.Node{r.Node}, segments, false, nil)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, fmt.Errorf("must be only one service record")
	}
	return &records[0], nil
}

// get is a wrapper for client.Get that uses SingleInflight to suppress multiple
// outstanding queries.
func (g *Backend) get(path string, recursive bool) (*etcdc.Response, error) {
	resp, err := g.inflight.Do(path, func() (interface{}, error) {
		r, e := g.client.Get(g.ctx, path, &etcdc.GetOptions{Sort: false, Recursive: recursive})
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

type bareService struct {
	Host     string
	Port     int
	Priority int
	Weight   int
	Text     string
}

// skydns/local/skydns/east/staging/web
// skydns/local/skydns/west/production/web
//
// skydns/local/skydns/*/*/web
// skydns/local/skydns/*/web

// loopNodes recursively loops through the nodes and returns all the values. The nodes' keyname
// will be match against any wildcards when star is true.
func (g *Backend) loopNodes(ns []*etcdc.Node, nameParts []string, star bool, bx map[bareService]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[bareService]bool)
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
		b := bareService{serv.Host, serv.Port, serv.Priority, serv.Weight, serv.Text}
		if _, ok := bx[b]; ok {
			continue
		}
		bx[b] = true

		serv.Key = n.Key
		serv.Ttl = g.calculateTtl(n, serv)
		if serv.Priority == 0 {
			serv.Priority = int(g.config.Priority)
		}
		sx = append(sx, *serv)
	}
	return sx, nil
}

// calculateTtl returns the smaller of the etcd TTL and the service's
// TTL. If neither of these are set (have a zero value), the server
// default is used.
func (g *Backend) calculateTtl(node *etcdc.Node, serv *msg.Service) uint32 {
	etcdTtl := uint32(node.TTL)

	if etcdTtl == 0 && serv.Ttl == 0 {
		return g.config.Ttl
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

// Client exposes the underlying Etcd client (used in tests).
func (g *Backend) Client() etcdc.KeysAPI {
	return g.client
}
