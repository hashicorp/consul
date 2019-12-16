// Package consul provides the consul   backend plugin.
package consul

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	"github.com/hashicorp/consul/api"
	"github.com/miekg/dns"
)

const (
	priority      = 10  // default priority when nothing is set
	ttl           = 300 // default ttl when nothing is set
	consulTimeout = 5 * time.Second
)

var errKeyNotFound = errors.New("key not found")

// Consul is a plugin talks to an consul cluster.
type Consul struct {
	Next       plugin.Handler
	Fall       fall.F
	Zones      []string
	PathPrefix string
	Token      string
	Address    string
	Upstream   *upstream.Upstream
	Client     *api.Client
}

// Services implements the ServiceBackend interface.
func (c *Consul) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) ([]msg.Service, error) {
	services, err := c.Records(ctx, state, exact)
	if err != nil {
		return services, err
	}
	services = msg.Group(services)
	return services, err
}

// Reverse implements the ServiceBackend interface.
func (c *Consul) Reverse(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (services []msg.Service, err error) {
	return c.Services(ctx, state, exact, opt)
}

// Lookup implements the ServiceBackend interface.
func (c *Consul) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return c.Upstream.Lookup(ctx, state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (c *Consul) IsNameError(err error) bool {
	return err == errKeyNotFound
}

// Records looks up records in consul. If exact is true, it will lookup just this
// name. This is used when find matches when completing SRV lookups for instance.
func (c *Consul) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	name := state.Name()
	path, star := msg.PathWithWildcard(name, c.PathPrefix)
	r, wildcard, _, err := c.get(ctx, strings.Replace(path, "/", "", 1), !exact)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(msg.Path(name, c.PathPrefix), "/")
	//return c.loopNodes(r.Kvs, segments, star, state.QType())
	return c.loopNodes(r, segments, wildcard, star, state.QType())
}

func (c *Consul) get(ctx context.Context, path string, recursive bool) ([]*api.KVPair, bool, *api.QueryMeta, error) {
	_, cancel := context.WithTimeout(ctx, 5)
	defer cancel()
	q := &api.QueryOptions{}
	wildcard := false
	if recursive {
		if !strings.HasSuffix(path, "/") {
			path = path + "/"
		}
		r, meta, err := c.Client.KV().List(path, q)
		if err != nil {
			return nil, wildcard, meta, err
		}
		if len(r) == 0 {
			path = strings.TrimSuffix(path, "/")
			var r1 *api.KVPair
			r1, meta, err = c.Client.KV().Get(path, q)
			r = []*api.KVPair{}
			if r1 != nil && r1.Value != nil {
				r = append(r, r1)
			}
			if err != nil {
				return r, wildcard, meta, err
			}
			if len(r) == 0 {
				pathArr := strings.Split(path, "/")
				pathArr[len(pathArr)-1] = "any"
				path = strings.Join(pathArr, "/")
				r1, meta, err = c.Client.KV().Get(path, q)
				if r1 != nil && r1.Value != nil {
					r = append(r, r1)
				}
				wildcard = true
				if err != nil {
					return r, wildcard, meta, err
				}
				if len(r) == 0 {
					err = errKeyNotFound
					return r, wildcard, meta, err
				}
				return r, wildcard, meta, err
			}
		}
		return r, wildcard, meta, err
	}

	r, meta, err := c.Client.KV().Get(path, q)
	if err != nil {
		return []*api.KVPair{r}, wildcard, meta, err
	}
	if r == nil {
		return nil, wildcard, meta, errKeyNotFound
	}
	return []*api.KVPair{r}, wildcard, meta, err
}

func (c *Consul) loopNodes(kv []*api.KVPair, nameParts []string, wildcard bool, star bool, qType uint16) (sx []msg.Service, err error) {
	bx := make(map[msg.Service]struct{})
Nodes:
	for _, n := range kv {
		keyParts := strings.Split(string(n.Key), "/")
		if star {
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
		} else {
			for _, n := range keyParts {
				if (n == "*" || n == "any") && !wildcard {
					continue Nodes
				}
			}
		}

		serv := new(msg.Service)
		if n.Value != nil {
			if err := json.Unmarshal(n.Value, serv); err != nil {
				return nil, fmt.Errorf("%s: %s", n.Key, err.Error())
			}
		} else {
			continue
		}
		serv.Key = string(n.Key)
		if _, ok := bx[*serv]; ok {
			continue
		}
		if !strings.HasPrefix(serv.Key, "/") {
			serv.Key = "/" + serv.Key
		}
		bx[*serv] = struct{}{}

		serv.TTL = c.TTL(n, serv)
		if serv.Priority == 0 {
			serv.Priority = priority
		}

		if shouldInclude(serv, qType) {
			sx = append(sx, *serv)
		}
	}
	return sx, nil
}

// TTL returns the smaller of the consul TTL and the service's
// TTL. If neither of these are set (have a zero value), a default is used.
func (c *Consul) TTL(kv *api.KVPair, serv *msg.Service) uint32 {
	consulTTL := uint32(serv.TTL)
	if consulTTL == 0 {
		return ttl
	}
	return serv.TTL
}

// shouldInclude returns true if the service should be included in a list of records, given the qType. For all the
// currently supported lookup types, the only one to allow for an empty Host field in the service are TXT records.
// Similarly, the TXT record in turn requires the Text field to be set.
func shouldInclude(serv *msg.Service, qType uint16) bool {
	if qType == dns.TypeTXT {
		return serv.Text != ""
	}
	return serv.Host != ""
}
