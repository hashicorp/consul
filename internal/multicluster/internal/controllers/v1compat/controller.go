// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package v1compat

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ControllerName    = "consul.io/exported-services-v1-compat"
	controllerMetaKey = "managed-by-controller"
)

type ConfigEntry interface {
	GetExportedServicesConfigEntry(context.Context, string, *acl.EnterpriseMeta) (*structs.ExportedServicesConfigEntry, error)
	WriteExportedServicesConfigEntry(context.Context, *structs.ExportedServicesConfigEntry) error
	DeleteExportedServicesConfigEntry(context.Context, string, *acl.EnterpriseMeta) error
}

func mapExportedServices(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	return []controller.Request{
		{
			ID: &pbresource.ID{
				Type: pbmulticluster.ComputedExportedServicesType,
				Tenancy: &pbresource.Tenancy{
					Partition: res.Id.Tenancy.Partition,
				},
				Name: types.ComputedExportedServicesName,
			},
		},
	}, nil
}

func Controller(config ConfigEntry) controller.Controller {
	return controller.ForType(pbmulticluster.ComputedExportedServicesType).
		WithWatch(pbmulticluster.PartitionExportedServicesType, mapExportedServices).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, mapExportedServices).
		WithWatch(pbmulticluster.ExportedServicesType, mapExportedServices).
		// TODO Add custom watch for exported-services for config entry events to attempt re-reconciliation when that changes
		WithReconciler(&reconciler{config: config})
}

type reconciler struct {
	config ConfigEntry
}

// Reconcile will reconcile one ComputedExportedServices in response to some event.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	entMeta := acl.DefaultEnterpriseMeta()
	entMeta.OverridePartition(req.ID.Tenancy.Partition)
	existing, err := r.config.GetExportedServicesConfigEntry(ctx, req.ID.Tenancy.Partition, entMeta)
	if err != nil {
		rt.Logger.Error("error getting exported service config entry", "error", err)
	}

	if existing != nil && existing.Meta["managed-by-controller"] != ControllerName {
		// in order to not cause an outage we need to ensure that the user has replicated all
		// of their exports to v2 resources first and then setting the meta key on the existing
		// config entry to allow this controller to overwrite what the user previously had.
		rt.Logger.Info("existing exported-services config entry is not managed with v2 resources. Add a metadata value of %q with a value of %q to opt-in to controller management of that config entry", controllerMetaKey, ControllerName)
		return nil
	}

	newCfg := &structs.ExportedServicesConfigEntry{
		// v1 exported-services config entries must have a Name that is the partitions name
		Name: req.ID.Tenancy.Partition,
		Meta: map[string]string{
			controllerMetaKey: ControllerName,
		},
		EnterpriseMeta: *entMeta,
	}

	partitionExports, err := resource.ListDecodedResource[*pbmulticluster.PartitionExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbmulticluster.PartitionExportedServicesType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving partition exported services", "error", err)
		return err
	}

	namespaceExports, err := resource.ListDecodedResource[*pbmulticluster.NamespaceExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbmulticluster.NamespaceExportedServicesType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving namespace exported service", "error", err)
		return err
	}

	serviceExports, err := resource.ListDecodedResource[*pbmulticluster.ExportedServices](ctx, rt.Client, &pbresource.ListRequest{
		Type:    pbmulticluster.ExportedServicesType,
		Tenancy: req.ID.Tenancy,
	})

	if err != nil {
		rt.Logger.Error("error retrieving exported services", "error", err)
		return err
	}

	if len(partitionExports) == 0 && len(namespaceExports) == 0 && len(serviceExports) == 0 {
		if existing == nil {
			return nil
		}

		if err := r.config.DeleteExportedServicesConfigEntry(ctx, req.ID.Tenancy.Partition, entMeta); err != nil {
			rt.Logger.Error("error deleting exported services config entry", "error", err)
			return err
		}
		return nil
	}

	exports := newExportTracker()

	for _, partitionExport := range partitionExports {
		exports.trackPartitionConsumer(partitionExport.Data.Consumers)
	}

	for _, namespaceExport := range namespaceExports {
		exports.trackNamespaceConsumers(namespaceExport.Id.Tenancy.Namespace, namespaceExport.Data.Consumers)
	}

	for _, serviceExport := range serviceExports {
		for _, svc := range serviceExport.Data.Services {
			svcId := &pbresource.ID{
				Type:    pbcatalog.ServiceType,
				Tenancy: serviceExport.Id.Tenancy,
				Name:    svc,
			}

			exports.trackExportedServices(svcId, serviceExport.Data.Consumers)
		}
	}

	newCfg.Services = exports.allExports()

	if existing != nil && configEntryEquivalent(existing, newCfg) {
		rt.Logger.Trace("managed exported-services config entry is already up to date")
		return nil
	}

	if err := r.config.WriteExportedServicesConfigEntry(ctx, newCfg); err != nil {
		rt.Logger.Error("error writing exported services config entry", "error", err)
		return err
	}

	rt.Logger.Debug("Updated exported services config entry")
	return nil
}

type exportTracker struct {
	partitions *exportConsumers
	namespaces map[string]*exportConsumers
	services   map[resource.ReferenceKey]*exportConsumers
}

type exportConsumers struct {
	partitions     map[string]struct{}
	peers          map[string]struct{}
	samenessGroups map[string]struct{}
}

func newExportConsumers() *exportConsumers {
	return &exportConsumers{
		partitions:     make(map[string]struct{}),
		peers:          make(map[string]struct{}),
		samenessGroups: make(map[string]struct{}),
	}
}

func (c *exportConsumers) addConsumers(consumers []*pbmulticluster.ExportedServicesConsumer) {
	for _, consumer := range consumers {
		switch v := consumer.ConsumerTenancy.(type) {
		case *pbmulticluster.ExportedServicesConsumer_Peer:
			c.peers[v.Peer] = struct{}{}
		case *pbmulticluster.ExportedServicesConsumer_Partition:
			c.partitions[v.Partition] = struct{}{}
		case *pbmulticluster.ExportedServicesConsumer_SamenessGroup:
			c.samenessGroups[v.SamenessGroup] = struct{}{}
		default:
			panic(fmt.Errorf("Unknown exported service consumer type: %T", v))
		}
	}
}

func (c *exportConsumers) configEntryConsumers() []structs.ServiceConsumer {
	consumers := make([]structs.ServiceConsumer, 0, len(c.partitions)+len(c.peers)+len(c.samenessGroups))

	partitions := keys(c.partitions)
	slices.Sort(partitions)
	for _, consumer := range partitions {
		consumers = append(consumers, structs.ServiceConsumer{
			Partition: consumer,
		})
	}

	peers := keys(c.peers)
	slices.Sort(peers)
	for _, consumer := range peers {
		consumers = append(consumers, structs.ServiceConsumer{
			Partition: consumer,
		})
	}

	samenessGroups := keys(c.samenessGroups)
	slices.Sort(samenessGroups)
	for _, consumer := range samenessGroups {
		consumers = append(consumers, structs.ServiceConsumer{
			Partition: consumer,
		})
	}

	return consumers
}

func newExportTracker() *exportTracker {
	return &exportTracker{
		partitions: newExportConsumers(),
		namespaces: make(map[string]*exportConsumers),
		services:   make(map[resource.ReferenceKey]*exportConsumers),
	}
}

func (t *exportTracker) trackPartitionConsumer(consumers []*pbmulticluster.ExportedServicesConsumer) {
	t.partitions.addConsumers(consumers)
}

func (t *exportTracker) trackNamespaceConsumers(namespace string, consumers []*pbmulticluster.ExportedServicesConsumer) {
	c, ok := t.namespaces[namespace]
	if !ok {
		c = newExportConsumers()
		t.namespaces[namespace] = c
	}

	c.addConsumers(consumers)
}

func (t *exportTracker) trackExportedServices(svcID *pbresource.ID, consumers []*pbmulticluster.ExportedServicesConsumer) {
	key := resource.NewReferenceKey(svcID)

	c, ok := t.services[key]
	if !ok {
		c = newExportConsumers()
		t.services[key] = c
	}

	c.addConsumers(consumers)
}

func (t *exportTracker) allExports() []structs.ExportedService {
	var exports []structs.ExportedService

	partitionConsumers := t.partitions.configEntryConsumers()
	if len(partitionConsumers) > 0 {
		exports = append(exports, structs.ExportedService{
			Name:      "*",
			Namespace: "*",
			Consumers: partitionConsumers,
		})
	}

	namespaces := keys(t.namespaces)
	slices.Sort(namespaces)
	for _, ns := range namespaces {
		exports = append(exports, structs.ExportedService{
			Name:      "*",
			Namespace: ns,
			Consumers: t.namespaces[ns].configEntryConsumers(),
		})
	}

	services := keys(t.services)
	slices.SortFunc(services, func(a, b resource.ReferenceKey) int {
		// the partitions must already be equal because we are only
		// looking at resource exports for a single partition.

		if a.Namespace < b.Namespace {
			return -1
		} else if a.Namespace > b.Namespace {
			return 1
		}

		if a.Name < b.Name {
			return -1
		} else if a.Name > b.Name {
			return 1
		}

		return 0
	})
	for _, svcKey := range services {
		exports = append(exports, structs.ExportedService{
			Name:      svcKey.Name,
			Namespace: svcKey.Namespace,
			Consumers: t.services[svcKey].configEntryConsumers(),
		})
	}

	return exports
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

func keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}
