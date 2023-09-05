// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package sentinel

import (
	"github.com/hashicorp/consul/api"
)

// ScopeFn is a callback that provides a sentinel scope. This is a callback
// so that if we don't run sentinel for some reason (not enabled or a basic
// policy check means we don't have to) then we don't spend the effort to make
// the map.
type ScopeFn func() map[string]interface{}

// ScopeKVUpsert returns the standard sentinel scope for a KV create or update.
func ScopeKVUpsert(key string, value []byte, flags uint64) map[string]interface{} {
	return map[string]interface{}{
		"key":   key,
		"value": string(value),
		"flags": flags,
	}
}

// ScopeCatalogUpsert returns the standard sentinel scope for a catalog create
// or update. Service is allowed to be nil.
func ScopeCatalogUpsert(node *api.Node, service *api.AgentService) map[string]interface{} {
	return map[string]interface{}{
		"node":    node,
		"service": service,
	}
}
