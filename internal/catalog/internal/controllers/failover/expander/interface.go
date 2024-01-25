// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// SamenessgroupExpander is used to expand sameness group for a ComputedFailover resource
type SamenessGroupExpander interface {
	ComputeFailoverDestinationsFromSamenessGroup(rt controller.Runtime, id *pbresource.ID, sg string, port string) ([]*pbcatalog.FailoverDestination, string, error)
	GetSamenessGroupIndex() *index.Index
}
