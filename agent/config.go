package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var errInvalidHeaderFormat = errors.New("agent: invalid format of 'header' field")

func FixupCheckType(raw interface{}) error {
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

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

	replace := func(oldKey, newKey string, val interface{}) {
		rawMap[newKey] = val
		if oldKey != newKey {
			delete(rawMap, oldKey)
		}
	}

	for k, v := range rawMap {
		switch strings.ToLower(k) {
		case "header":
			h, err := parseHeaderMap(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %s", k, err)
			}
			rawMap[k] = h

		case "ttl", "interval", "timeout":
			d, err := parseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %v", k, err)
			}
			rawMap[k] = d

		case "deregister_critical_service_after", "deregistercriticalserviceafter":
			d, err := parseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %v", k, err)
			}
			replace(k, "DeregisterCriticalServiceAfter", d)

		case "docker_container_id":
			replace(k, "DockerContainerID", v)

		case "service_id":
			replace(k, "ServiceID", v)

		case "tls_skip_verify":
			replace(k, "TLSSkipVerify", v)
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
