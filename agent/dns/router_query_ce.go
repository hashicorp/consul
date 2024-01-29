// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"errors"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/discovery"
)

// getQueryTenancy returns a discovery.QueryTenancy from a DNS message.
func getQueryTenancyForService(querySuffixes []string,
	defaultEntMeta acl.EnterpriseMeta, cfg *RouterDynamicConfig, defaultDatacenter string) (discovery.QueryTenancy, error) {
	locality, ok := discovery.ParseLocality(querySuffixes, defaultEntMeta, cfg.EnterpriseDNSConfig)
	if !ok {
		return discovery.QueryTenancy{}, errors.New("invalid locality")
	}

	return discovery.GetQueryTenancyBasedOnLocality(locality, defaultDatacenter)
}
