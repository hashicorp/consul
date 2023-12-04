// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package trafficpermissions

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// getEnterpriseTrafficPermissions is a stub for enterprise traffic permissions.
// We return empty allow and deny slices.
func getEnterpriseTrafficPermissions(
	_ context.Context,
	_ controller.Runtime,
	_ *pbresource.ID,
) (allow []*pbauth.Permission, deny []*pbauth.Permission, err error) {
	allow = make([]*pbauth.Permission, 0)
	deny = make([]*pbauth.Permission, 0)
	return allow, deny, nil
}
