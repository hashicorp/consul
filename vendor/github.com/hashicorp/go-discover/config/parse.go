// Package config provides functions for handling configuration.
package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Parse parses a "key=val key=val ..." string into
// a string map. Values are URL escaped.
func Parse(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	m := map[string]string{}
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
		m[key] = val
	}
	return m, nil
}
