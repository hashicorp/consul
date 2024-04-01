// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package testing

import (
	"context"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/internal/storage/inmem"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

func FillEntMeta(entMeta *acl.EnterpriseMeta) {
	// nothing to to in CE.
}

func FillAuthorizerContext(authzContext *acl.AuthorizerContext) {
	// nothing to to in CE.
}

// initTenancy creates the builtin v2 namespace resource only. The builtin
// v2 partition is not created because we're in CE.
func initTenancy(ctx context.Context, b *inmem.Backend) error {
	nsData, err := anypb.New(&pbtenancy.Namespace{Description: "default namespace in default partition"})
	if err != nil {
		return err
	}
	nsID := &pbresource.ID{
		Type:    pbtenancy.NamespaceType,
		Name:    resource.DefaultNamespaceName,
		Tenancy: resource.DefaultPartitionedTenancy(),
		Uid:     ulid.Make().String(),
	}
	read, err := b.Read(ctx, storage.StrongConsistency, nsID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if read == nil && errors.Is(err, storage.ErrNotFound) {
		_, err = b.WriteCAS(ctx, &pbresource.Resource{
			Id:         nsID,
			Generation: ulid.Make().String(),
			Data:       nsData,
			Metadata: map[string]string{
				"generated_at": time.Now().Format(time.RFC3339),
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
