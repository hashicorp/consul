package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/config"
)

var errInvalidHeaderFormat = errors.New("agent: invalid format of 'header' field")

func FixupCheckType(raw interface{}) error {
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	// See https://github.com/hashicorp/consul/pull/3557 why we need this
	// and why we should get rid of it. In Consul 1.0 we also didn't map
	// Args correctly, so we ended up exposing (and need to carry forward)
	// ScriptArgs, see https://github.com/hashicorp/consul/issues/3587.
	config.TranslateKeys(rawMap, map[string]string{
		"args":                              "ScriptArgs",
		"script_args":                       "ScriptArgs",
		"deregister_critical_service_after": "DeregisterCriticalServiceAfter",
		"docker_container_id":               "DockerContainerID",
		"tls_skip_verify":                   "TLSSkipVerify",
		"service_id":                        "ServiceID",
	})

	parseDuration := func(v interface{}) (time.Duration, error) {
		if v == nil {
			return 0, nil
		}
		switch x := v.(type) {
		case time.Duration:
			return x, nil
		case float64:
			return time.Duration(x), nil
		case string:
			return time.ParseDuration(x)
		default:
			return 0, fmt.Errorf("invalid format")
		}
	}

	parseHeaderMap := func(v interface{}) (map[string][]string, error) {
		if v == nil {
			return nil, nil
		}
		vm, ok := v.(map[string]interface{})
		if !ok {
			return nil, errInvalidHeaderFormat
		}
		m := map[string][]string{}
		for k, vv := range vm {
			vs, ok := vv.([]interface{})
			if !ok {
				return nil, errInvalidHeaderFormat
			}
			for _, vs := range vs {
				s, ok := vs.(string)
				if !ok {
					return nil, errInvalidHeaderFormat
				}
				m[k] = append(m[k], s)
			}
		}
		return m, nil
	}

	for k, v := range rawMap {
		switch strings.ToLower(k) {
		case "header":
			h, err := parseHeaderMap(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %s", k, err)
			}
			rawMap[k] = h

		case "ttl", "interval", "timeout", "deregistercriticalserviceafter":
			d, err := parseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %v", k, err)
			}
			rawMap[k] = d
		}
	}
	return nil
}

func ParseMetaPair(raw string) (string, string) {
	pair := strings.SplitN(raw, ":", 2)
	if len(pair) == 2 {
		return pair[0], pair[1]
	}
	return pair[0], ""
}
