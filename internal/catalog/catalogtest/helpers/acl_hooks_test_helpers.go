// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helpers

import (
	"testing"

	"github.com/hashicorp/consul/internal/catalog/internal/testhelpers"
	"github.com/hashicorp/consul/internal/catalog/workloadselector"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RunWorkloadSelectingTypeACLsTests[T workloadselector.WorkloadSelecting](t *testing.T, typ *pbresource.Type,
	getData func(selector *pbcatalog.WorkloadSelector) T,
	registerFunc func(registry resource.Registry),
) {
	testhelpers.RunWorkloadSelectingTypeACLsTests[T](t, typ, getData, registerFunc)
}
