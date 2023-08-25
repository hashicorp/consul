// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pbsubscribe

import (
	"time"

	"github.com/hashicorp/consul/acl"
)

// RequestDatacenter implements structs.RPCInfo
func (req *SubscribeRequest) RequestDatacenter() string {
	return req.Datacenter
}

// IsRead implements structs.RPCInfo
func (req *SubscribeRequest) IsRead() bool {
	return true
}

// AllowStaleRead implements structs.RPCInfo
func (req *SubscribeRequest) AllowStaleRead() bool {
	return true
}

// TokenSecret implements structs.RPCInfo
func (req *SubscribeRequest) TokenSecret() string {
	return req.Token
}

// SetTokenSecret implements structs.RPCInfo
func (req *SubscribeRequest) SetTokenSecret(token string) {
	req.Token = token
}

// HasTimedOut implements structs.RPCInfo
func (req *SubscribeRequest) HasTimedOut(start time.Time, rpcHoldTimeout, _, _ time.Duration) (bool, error) {
	return time.Since(start) > rpcHoldTimeout, nil
}

// EnterpriseMeta returns the EnterpriseMeta encoded in the request's Subject.
func (req *SubscribeRequest) EnterpriseMeta() acl.EnterpriseMeta {
	if req.GetWildcardSubject() {
		// Note: EnterpriseMeta is ignored for the wildcard subject (as it will
		// receive all events in the topic regardless of partition, namespace etc).
		return acl.EnterpriseMeta{}
	}

	if sub := req.GetNamedSubject(); sub != nil {
		return acl.NewEnterpriseMetaWithPartition(sub.Partition, sub.Namespace)
	}

	// Deprecated top-level Partition and Namespace fields.
	return acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)
}
