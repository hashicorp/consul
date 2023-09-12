// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/mappers/bimapper"
)

// LeafMapper is a wrapper around endpointsMapper to allow mapping from events to requests for PSTs (as opposed to from a resource to requests for PSTs).
type LeafMapper struct {
	*bimapper.Mapper
}

func (m *LeafMapper) EventMapLink(_ context.Context, _ controller.Runtime, event controller.Event) ([]controller.Request, error) {
	// Get cert from event.
	cert, ok := event.Obj.(*structs.IssuedCert)
	if !ok {
		return nil, fmt.Errorf("got invalid event type; expected *structs.IssuedCert")
	}

	// The LeafMapper has mappings from leaf certificate resource references to PSTs. So we need to translate the
	// contents of the certificate from the event to a leaf resource reference.
	leafRef := leafResourceRef(cert.WorkloadIdentity, cert.EnterpriseMeta.NamespaceOrDefault(), cert.EnterpriseMeta.PartitionOrDefault())

	// Get all the ProxyStateTemplates that reference this leaf.
	itemIDs := m.ItemIDsForLink(leafRef)
	out := make([]controller.Request, 0, len(itemIDs))

	for _, item := range itemIDs {
		out = append(out, controller.Request{ID: item})
	}
	return out, nil
}
