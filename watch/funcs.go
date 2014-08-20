package watch

import (
	"fmt"
	"strconv"

	"github.com/armon/consul-api"
)

// watchFactory is a function that can create a new WatchFunc
// from a parameter configuration
type watchFactory func(params map[string][]string) (WatchFunc, error)

// watchFuncFactory maps each type to a factory function
var watchFuncFactory map[string]watchFactory

func init() {
	watchFuncFactory = map[string]watchFactory{
		"key":       keyWatch,
		"keyprefix": keyPrefixWatch,
		"services":  servicesWatch,
		"nodes":     nodesWatch,
		"service":   serviceWatch,
		"checks":    checksWatch,
	}
}

// keyWatch is used to return a key watching function
func keyWatch(params map[string][]string) (WatchFunc, error) {
	var key string
	if err := assignValue(params, "key", &key); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, fmt.Errorf("Must specify a single key to watch")
	}

	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		kv := p.client.KV()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		pair, meta, err := kv.Get(key, &opts)
		if err != nil {
			return 0, nil, err
		}
		if pair == nil {
			return meta.LastIndex, nil, err
		}
		return meta.LastIndex, pair, err
	}
	return fn, nil
}

// keyPrefixWatch is used to return a key prefix watching function
func keyPrefixWatch(params map[string][]string) (WatchFunc, error) {
	var prefix string
	if err := assignValue(params, "prefix", &prefix); err != nil {
		return nil, err
	}
	if prefix == "" {
		return nil, fmt.Errorf("Must specify a single prefix to watch")
	}

	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		kv := p.client.KV()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		pairs, meta, err := kv.List(prefix, &opts)
		if err != nil {
			return 0, nil, err
		}
		return meta.LastIndex, pairs, err
	}
	return fn, nil
}

// servicesWatch is used to watch the list of available services
func servicesWatch(params map[string][]string) (WatchFunc, error) {
	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		catalog := p.client.Catalog()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		services, meta, err := catalog.Services(&opts)
		if err != nil {
			return 0, nil, err
		}
		return meta.LastIndex, services, err
	}
	return fn, nil
}

// nodesWatch is used to watch the list of available nodes
func nodesWatch(params map[string][]string) (WatchFunc, error) {
	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		catalog := p.client.Catalog()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		nodes, meta, err := catalog.Nodes(&opts)
		if err != nil {
			return 0, nil, err
		}
		return meta.LastIndex, nodes, err
	}
	return fn, nil
}

// serviceWatch is used to watch a specific service for changes
func serviceWatch(params map[string][]string) (WatchFunc, error) {
	var service, tag, passingRaw string
	if err := assignValue(params, "service", &service); err != nil {
		return nil, err
	}
	if service == "" {
		return nil, fmt.Errorf("Must specify a single service to watch")
	}

	if err := assignValue(params, "tag", &tag); err != nil {
		return nil, err
	}

	if err := assignValue(params, "passingonly", &passingRaw); err != nil {
		return nil, err
	}
	passingOnly := false
	if passingRaw != "" {
		b, err := strconv.ParseBool(passingRaw)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse passingonly value: %v", err)
		}
		passingOnly = b
	}

	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		health := p.client.Health()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		nodes, meta, err := health.Service(service, tag, passingOnly, &opts)
		if err != nil {
			return 0, nil, err
		}
		return meta.LastIndex, nodes, err
	}
	return fn, nil
}

// checksWatch is used to watch a specific checks in a given state
func checksWatch(params map[string][]string) (WatchFunc, error) {
	var service, state string
	if err := assignValue(params, "service", &service); err != nil {
		return nil, err
	}
	if err := assignValue(params, "state", &state); err != nil {
		return nil, err
	}
	if service != "" && state != "" {
		return nil, fmt.Errorf("Cannot specify service and state")
	}
	if service == "" && state == "" {
		state = "any"
	}

	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		health := p.client.Health()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		var checks []*consulapi.HealthCheck
		var meta *consulapi.QueryMeta
		var err error
		if state != "" {
			checks, meta, err = health.State(state, &opts)
		} else {
			checks, meta, err = health.Checks(service, &opts)
		}
		if err != nil {
			return 0, nil, err
		}
		return meta.LastIndex, checks, err
	}
	return fn, nil
}
