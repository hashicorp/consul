package discover

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Config stores key/value pairs for the discovery
// functions to use.
type Config map[string]string

// Parse parses a "key=val key=val ..." string into
// a config map. Values are URL escaped.
func Parse(s string) (Config, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	c := Config{}
	for _, v := range strings.Fields(s) {
		p := strings.SplitN(v, "=", 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("invalid format: %s", v)
		}
		key := p[0]
		val, err := url.QueryUnescape(p[1])
		if err != nil {
			return nil, fmt.Errorf("invalid format: %s", v)
		}
		c[key] = val
	}
	return c, nil
}

// String formats a config map into the "key=val key=val ..."
// understood by Parse. The order of the keys is stable.
func (c Config) String() string {
	// sort 'provider' to the front and keep the keys stable.
	var keys []string
	for k := range c {
		if k != "provider" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	keys = append([]string{"provider"}, keys...)

	var vals []string
	for _, k := range keys {
		v := c[k]
		if v == "" {
			continue
		}
		v = k + "=" + url.QueryEscape(v)
		vals = append(vals, v)
	}
	return strings.Join(vals, " ")
}
