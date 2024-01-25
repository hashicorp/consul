// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package expander_ce

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type SamenessGroupExpander struct{}

func New() *SamenessGroupExpander {
	return &SamenessGroupExpander{}
}

func (sgE *SamenessGroupExpander) ComputeFailoverDestinationsFromSamenessGroup(rt controller.Runtime, id *pbresource.ID, sg string, port string) ([]*pbcatalog.FailoverDestination, string, error) {
	//no - op for CE
	return nil, "", nil
}

const sgIndexName = "samenessGroupIndex"

func (sgE *SamenessGroupExpander) GetSamenessGroupIndex() *index.Index {
	return indexers.DecodedMultiIndexer(
		sgIndexName,
		index.ReferenceOrIDFromArgs,
		func(r *resource.DecodedResource[*pbcatalog.FailoverPolicy]) (bool, [][]byte, error) {
			//no - op for CE
			return false, nil, nil

		},
	)
}
