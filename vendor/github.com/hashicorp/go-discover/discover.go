// Package discover provides functions to get metadata for different
// cloud environments.
package discover

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
)

type Provider interface {
	Addrs(args map[string]string, l *log.Logger) ([]string, error)
}

var (
	mu        sync.Mutex
	providers = map[string]Provider{}
	helps     = map[string]string{}
)

// Register adds a new provider for the given name. If the
// provider is nil or already registered the function panics.
func Register(name string, provider Provider, help string) {
	mu.Lock()
	defer mu.Unlock()
	if provider == nil {
		panic("discover: Register called with nil provider")
	}
	if _, dup := providers[name]; dup {
		panic("discover: Register called twice for provider " + name)
	}
	providers[name] = provider
	helps[name] = help
}

// ProviderNames returns the names of the registered providers.
func ProviderNames() []string {
	var names []string
	for n := range helps {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

var globalHelp = `The options for discovering ip addresses are provided as a
  single string value in "key=value key=value ..." format where
  the values are URL encoded.

    provider=aws region=eu-west-1 ...

  The options are provider specific and are listed below.
`

// Help describes the format of the configuration string for address discovery
// and the various provider specific options.
func Help() string {
	mu.Lock()
	defer mu.Unlock()
	h := []string{globalHelp}
	for _, name := range ProviderNames() {
		h = append(h, helps[name])
	}
	return strings.Join(h, "\n")
}

// Addrs discovers ip addresses of nodes that match the given filter criteria.
// The config string must have the format 'provider=xxx key=val key=val ...'
// where the keys and values are provider specific. The values are URL encoded.
func Addrs(cfg string, l *log.Logger) ([]string, error) {
	args, err := Parse(cfg)
	if err != nil {
		return nil, fmt.Errorf("discover: %s", err)
	}

	name := args["provider"]
	if name == "" {
		return nil, fmt.Errorf("discover: no provider")
	}

	mu.Lock()
	p := providers[name]
	mu.Unlock()

	if p == nil {
		return nil, fmt.Errorf("discover: unknown provider " + name)
	}
	return p.Addrs(args, l)
}
