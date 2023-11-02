// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package v1compat

import (
	"context"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
)

// func mapConfigEntry(ctx context.Context, rt controller.Runtime, e controller.Event) ([]controller.Request, error) {
// 	return nil, nil
// }

type ConfigEntry interface {
	GetExportedServicesConfigEntry(context.Context, string, *acl.EnterpriseMeta) (*structs.ExportedServicesConfigEntry, error)
	WriteExportedServicesConfigEntry(context.Context, *structs.ExportedServicesConfigEntry) error
	DeleteExportedServicesConfigEntry(context.Context, string, *acl.EnterpriseMeta) error
}

func Controller(config ConfigEntry) controller.Controller {
	// configEntryEvents := make(chan controller.Event, 1000)

	return controller.ForType(pbmulticluster.ComputedExportedServicesType).
		// WithCustomWatch(&controller.Source{Source: configEntryEvents}, mapConfigEntry).
		WithReconciler(&reconciler{config: config})
}

type reconciler struct {
	config ConfigEntry
}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "name", "exports-services-v1-compat")
	res, err := resource.GetDecodedResource[*pbmulticluster.ComputedExportedServices](ctx, rt.Client, req.ID)
	if err != nil {
		rt.Logger.Error("error getting computed exported services", "error", err)
		return err
	}

	entMeta := acl.DefaultEnterpriseMeta()
	entMeta.OverridePartition(req.ID.Tenancy.Partition)
	cfg, err := r.config.GetExportedServicesConfigEntry(ctx, req.ID.Tenancy.Partition, entMeta)
	if err != nil {
		rt.Logger.Error("error getting exported service config entry", "error", err)
	}

	if res == nil && cfg != nil && cfg.Meta["managed-by-controller"] == "exported-services-v1-compat" {
		if err := r.config.DeleteExportedServicesConfigEntry(ctx, req.ID.Tenancy.Partition, entMeta); err != nil {
			rt.Logger.Error("error deleting exported services config entry", "error", err)
			return err
		}
		return nil
	}

	if res == nil {
		return nil
	}

	newCfg := newConfigEntryFromResource(res)

	if cfg != nil && configEntryEquivalent(cfg, newCfg) {
		return nil
	}

	if res != nil && (cfg == nil || !configEntryEquivalent(cfg, newCfg)) {
		if err := r.config.WriteExportedServicesConfigEntry(ctx, newCfg); err != nil {
			rt.Logger.Error("error writing exported services config entry", "error", err)
			return err
		}

		rt.Logger.Debug("Updated exported services config entry")
	}

	return nil
}

func newConfigEntryFromResource(res *resource.DecodedResource[*pbmulticluster.ComputedExportedServices]) *structs.ExportedServicesConfigEntry {
	entMeta := acl.DefaultEnterpriseMeta()
	entMeta.OverridePartition(res.Id.Tenancy.Partition)

	newCfg := &structs.ExportedServicesConfigEntry{
		Name: res.Id.Tenancy.Partition,
		Meta: map[string]string{
			"managed-by-controller": "exported-services-v1-compat",
		},
		EnterpriseMeta: *entMeta,
	}

	for _, svc := range res.Data.Consumers {
		exp := structs.ExportedService{
			Name:      svc.TargetRef.Name,
			Namespace: svc.TargetRef.Tenancy.Namespace,
		}

		for _, consumer := range svc.Consumers {
			exp.Consumers = append(exp.Consumers, structs.ServiceConsumer{
				Partition: consumer.GetPartition(),
				Peer:      consumer.GetPeer(),
			})
		}

		newCfg.Services = append(newCfg.Services, exp)
	}

	return newCfg
}

func configEntryEquivalent(a, b *structs.ExportedServicesConfigEntry) bool {
	if a.Name != b.Name {
		return false
	}

	if len(a.Services) != len(b.Services) {
		return false
	}

	for i := 0; i < len(a.Services); i++ {
		svcA := a.Services[i]
		svcB := b.Services[i]

		if svcA.Name != svcB.Name {
			return false
		}

		if svcA.Namespace != svcB.Namespace {
			return false
		}

		if len(svcA.Consumers) != len(svcB.Consumers) {
			return false
		}

		for j := 0; j < len(svcA.Consumers); j++ {
			consumerA := svcA.Consumers[j]
			consumerB := svcB.Consumers[j]

			if consumerA.Partition != consumerB.Partition {
				return false
			}

			if consumerA.Peer != consumerB.Peer {
				return false
			}

			if consumerA.SamenessGroup != consumerB.SamenessGroup {
				return false
			}
		}
	}
	return true
}
