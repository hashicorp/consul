package watch

import (
	"fmt"

	"github.com/armon/consul-api"
)

// watchFactory is a function that can create a new WatchFunc
// from a parameter configuration
type watchFactory func(params map[string][]string) (WatchFunc, error)

// watchFuncFactory maps each type to a factory function
var watchFuncFactory map[string]watchFactory

func init() {
	watchFuncFactory = map[string]watchFactory{
		"key": keyWatch,
	}
}

// keyWatch is used to return a key watching function
func keyWatch(params map[string][]string) (WatchFunc, error) {
	keys := params["key"]
	delete(params, "key")
	if len(keys) != 1 {
		return nil, fmt.Errorf("Must specify a single key to watch")
	}
	key := keys[0]

	fn := func(p *WatchPlan) (uint64, interface{}, error) {
		kv := p.client.KV()
		opts := consulapi.QueryOptions{WaitIndex: p.lastIndex}
		pair, meta, err := kv.Get(key, &opts)
		if err != nil {
			return 0, nil, err
		}
		return meta.LastIndex, pair, err
	}
	return fn, nil
}
