// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package trafficpermissions

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
)

const SgIndexName = "samenessGroupIndex"

func registerEnterpriseControllerWatchers(ctrl *controller.Controller) *controller.Controller {
	return ctrl
}

func GetSamenessGroupIndex() *index.Index {
	return indexers.DecodedMultiIndexer(
		SgIndexName,
		index.ReferenceOrIDFromArgs,
		func(r *resource.DecodedResource[*pbauth.TrafficPermissions]) (bool, [][]byte, error) {
			//no - op for CE
			return false, nil, nil

		},
	)
}
