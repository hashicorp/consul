// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
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
		case strings.Contains(strings.ToLower(k), "jwt"):
			err = multierror.Append(err, fmt.Errorf("invalid config key %q, api-gateway jwt validation is a consul enterprise feature", k))
		default:
			err = multierror.Append(err, fmt.Errorf("invalid config key %q", k))
		}
	}
	return err
}

func validateInnerEnterpriseMeta(_, _ *acl.EnterpriseMeta) error {
	return nil
}

func validateExportedServicesName(name string) error {
	if name != "default" {
		return fmt.Errorf(`exported-services Name must be "default"`)
	}
	return nil
}

func makeEnterpriseConfigEntry(kind, name string) ConfigEntry {
	return nil
}

func validateRatelimit(rl *RateLimits) error {
	if rl != nil {
		return fmt.Errorf("invalid rate_limits config. Rate limiting is a consul enterprise feature")
	}
	return nil
}

func (rl RateLimits) ToEnvoyExtension() *EnvoyExtension { return nil }

// GetLocalUpstreamIDs returns the list of non-peer service ids for upstreams defined on this request.
// This is often used for fetching service-defaults config entries.
func (s *ServiceConfigRequest) GetLocalUpstreamIDs() []ServiceID {
	var upstreams []ServiceID
	for _, u := range s.UpstreamServiceNames {
		if u.Peer != "" {
			continue
		}
		upstreams = append(upstreams, u.ServiceName.ToServiceID())
	}
	return upstreams
}
