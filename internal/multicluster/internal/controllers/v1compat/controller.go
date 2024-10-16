// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package v1compat

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"golang.org/x/exp/maps"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/multicluster/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ControllerName    = "consul.io/exported-services-v1-compat"
	controllerMetaKey = "managed-by-controller"
)

//go:generate mockery --name AggregatedConfig --inpackage --with-expecter --filename mock_AggregatedConfig.go
type AggregatedConfig interface {
	Start(context.Context)
	GetExportedServicesConfigEntry(context.Context, string, *acl.EnterpriseMeta) (*structs.ExportedServicesConfigEntry, error)
	WriteExportedServicesConfigEntry(context.Context, *structs.ExportedServicesConfigEntry) error
	DeleteExportedServicesConfigEntry(context.Context, string, *acl.EnterpriseMeta) error
	EventChannel() chan controller.Event
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

func mapConfigEntryEvents(ctx context.Context, rt controller.Runtime, event controller.Event) ([]controller.Request, error) {
	partition := event.Obj.Key()

	return []controller.Request{
		{
			ID: &pbresource.ID{
				Type: pbmulticluster.ComputedExportedServicesType,
				Tenancy: &pbresource.Tenancy{
					Partition: partition,
				},
				Name: types.ComputedExportedServicesName,
			},
		},
	}, nil
}

func Controller(config AggregatedConfig) *controller.Controller {
	return controller.NewController(ControllerName, pbmulticluster.ComputedExportedServicesType).
		WithNotifyStart(func(ctx context.Context, r controller.Runtime) {
			go config.Start(ctx)
		}).
		WithWatch(pbmulticluster.PartitionExportedServicesType, mapExportedServices).
		WithWatch(pbmulticluster.NamespaceExportedServicesType, mapExportedServices).
		WithWatch(pbmulticluster.ExportedServicesType, mapExportedServices).
		WithCustomWatch(&controller.Source{Source: config.EventChannel()}, mapConfigEntryEvents).
		WithReconciler(&reconciler{config: config})
}

type reconciler struct {
	config AggregatedConfig
}

// Reconcile will reconcile one ComputedExportedServices in response to some event.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	entMeta := acl.DefaultEnterpriseMeta()
	entMeta.OverridePartition(req.ID.Tenancy.Partition)
	existing, err := r.config.GetExportedServicesConfigEntry(ctx, req.ID.Tenancy.Partition, entMeta)
	if err != nil {
		// When we can't read the existing exported-services we purposely allow
		// reconciler to continue so we can still write a new one
		rt.Logger.Warn("error getting exported service config entry but continuing reconcile", "error", err)
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

	partitionExports, err := cache.ListDecoded[*pbmulticluster.PartitionExportedServices](
		rt.Cache,
		pbmulticluster.PartitionExportedServicesType,
		"id",
		&pbresource.ID{
			Type:    pbmulticluster.PartitionExportedServicesType,
			Tenancy: req.ID.Tenancy,
		},
		index.IndexQueryOptions{Prefix: true},
	)
	if err != nil {
		rt.Logger.Error("error retrieving partition exported services", "error", err)
		return err
	}

	namespaceExports, err := cache.ListDecoded[*pbmulticluster.NamespaceExportedServices](
		rt.Cache,
		pbmulticluster.NamespaceExportedServicesType,
		"id",
		&pbresource.ID{
			Type:    pbmulticluster.NamespaceExportedServicesType,
			Tenancy: req.ID.Tenancy,
		},
		index.IndexQueryOptions{Prefix: true},
	)
	if err != nil {
		rt.Logger.Error("error retrieving namespace exported services", "error", err)
		return err
	}

	serviceExports, err := cache.ListDecoded[*pbmulticluster.ExportedServices](
		rt.Cache,
		pbmulticluster.ExportedServicesType,
		"id",
		&pbresource.ID{
			Type:    pbmulticluster.ExportedServicesType,
			Tenancy: req.ID.Tenancy,
		},
		index.IndexQueryOptions{Prefix: true},
	)
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
				Type:    fakeServiceType,
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

	partitions := maps.Keys(c.partitions)
	slices.Sort(partitions)
	for _, consumer := range partitions {
		consumers = append(consumers, structs.ServiceConsumer{
			Partition: consumer,
		})
	}

	peers := maps.Keys(c.peers)
	slices.Sort(peers)
	for _, consumer := range peers {
		consumers = append(consumers, structs.ServiceConsumer{
			Peer: consumer,
		})
	}

	samenessGroups := maps.Keys(c.samenessGroups)
	slices.Sort(samenessGroups)
	for _, consumer := range samenessGroups {
		consumers = append(consumers, structs.ServiceConsumer{
			SamenessGroup: consumer,
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

	namespaces := maps.Keys(t.namespaces)
	slices.Sort(namespaces)
	for _, ns := range namespaces {
		exports = append(exports, structs.ExportedService{
			Name:      "*",
			Namespace: ns,
			Consumers: t.namespaces[ns].configEntryConsumers(),
		})
	}

	services := maps.Keys(t.services)
	sort.Slice(services, func(i, j int) bool {
		// the partitions must already be equal because we are only
		// looking at resource exports for a single partition.

		if services[i].Namespace < services[j].Namespace {
			return false
		} else if services[i].Namespace > services[j].Namespace {
			return true
		}

		if services[i].Name < services[j].Name {
			return false
		} else if services[i].Name > services[j].Name {
			return true
		}

		return false
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

var fakeServiceType = &pbresource.Type{
	Group:        "catalog",
	GroupVersion: "v2beta1",
	Kind:         "Service",
}
