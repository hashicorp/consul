package discover

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/hashicorp/consul/discover/aws"
	"github.com/hashicorp/consul/discover/azure"
	"github.com/hashicorp/consul/discover/gce"
)

// Discoverer defines a function which discovers ip addresses of nodes
// for a given configuration.
type Discoverer func(cfg map[string]string, l *log.Logger) ([]string, error)

// Discoverers is the list of available discoverers.
var Discoverers = map[string]Discoverer{}

func init() {
	Discoverers = map[string]Discoverer{
		"aws":   aws.Discover,
		"gce":   gce.Discover,
		"azure": azure.Discover,
	}
}

// Parse parses a "key=val key=val ..." config string into
// a string map. Values are URL escaped.
func Parse(cfg string) (map[string]string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		return nil, nil
	}

	m := map[string]string{}
	for _, v := range strings.Fields(cfg) {
		p := strings.SplitN(v, "=", 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("discover: invalid format: %s", v)
		}
		key := p[0]
		val, err := url.QueryUnescape(p[1])
		if err != nil {
			return nil, fmt.Errorf("discover: invalid format: %s", v)
		}
		m[key] = val
	}
	return m, nil
}

// Discover takes a generic configuration string as "key=val key=val ..."
// and discovers based on the provider value.
func Discover(cfg string, l *log.Logger) ([]string, error) {
	m, err := Parse(cfg)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	p := m["provider"]
	if p == "" {
		return nil, fmt.Errorf("discover: missing 'provider' value")
	}
	delete(m, "provider")
	d := Discoverers[p]
	if d == nil {
		return nil, fmt.Errorf("discover: unknown provider %q", p)
	}
	return d(m, l)
}
