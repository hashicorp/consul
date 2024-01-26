// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

// XTrafficPermissions is an interface to allow generic handling of
// TrafficPermissions, NamespaceTrafficPermissions, and PartitionTrafficPermissions.
type XTrafficPermissions interface {
	proto.Message

	GetAction() pbauth.Action
	GetPermissions() []*pbauth.Permission
}

func Register(r resource.Registry) {
	RegisterWorkloadIdentity(r)
	RegisterTrafficPermissions(r)
	RegisterComputedTrafficPermission(r)
	RegisterNamespaceTrafficPermissions(r)
	RegisterPartitionTrafficPermissions(r)
}
