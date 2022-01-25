//go:build !consulent
// +build !consulent

package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

func (e *ProxyConfigEntry) validateEnterpriseMeta() error {
	return nil
}

func validateUnusedKeys(unused []string) error {
	var err error

	for _, k := range unused {
		switch {
		case k == "CreateIndex" || k == "ModifyIndex":
		case k == "kind" || k == "Kind":
			// The kind field is used to determine the target, but doesn't need
			// to exist on the target.
		case strings.HasSuffix(strings.ToLower(k), "namespace"):
			err = multierror.Append(err, fmt.Errorf("invalid config key %q, namespaces are a consul enterprise feature", k))
		default:
			err = multierror.Append(err, fmt.Errorf("invalid config key %q", k))
		}
	}
	return err
}

func validateInnerEnterpriseMeta(_, _ *EnterpriseMeta) error {
	return nil
}

func requireEnterprise(kind string) error {
	return fmt.Errorf("Config entry kind %q requires Consul Enterprise", kind)
}
