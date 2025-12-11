// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"github.com/hashicorp/consul/agent/cacheshim"
)

// Request is a cacheable request.
//
// This interface is typically implemented by request structures in
// the agent/structs package.
//
//go:generate mockery --name Request --inpackage
type Request = cacheshim.Request

// RequestInfo represents cache information for a request. The caching
// framework uses this to control the behavior of caching and to determine
// cacheability.
//
// TODO(peering): finish ensuring everything that sets a Datacenter sets or doesn't set PeerName.
// TODO(peering): also make sure the peer name is present in the cache key likely in lieu of the datacenter somehow.
type RequestInfo = cacheshim.RequestInfo
