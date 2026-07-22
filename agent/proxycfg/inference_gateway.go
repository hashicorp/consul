// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
)

type handlerInferenceGateway struct {
	handlerState
}

// initialize sets up the watches needed for an inference gateway: the CA roots
// and the gateway's own leaf (for inbound mesh mTLS), the mesh config entry, and
// the bound ai-gateway config entry (the routing policy). Model upstream watches
// are added dynamically as the routing policy names candidate clusters.
func (s *handlerInferenceGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(s.serviceInstance, s.stateConfig)

	// Watch for root changes (inbound mTLS trust).
	err := s.dataSources.CARoots.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for root changes", "error", err)
		return snap, err
	}

	// Watch the gateway's own leaf cert; it terminates inbound mesh mTLS from
	// calling agents and is the gateway's SPIFFE identity.
	err = s.dataSources.LeafCertificate.Notify(ctx, &leafcert.ConnectCALeafRequest{
		Datacenter:     s.source.Datacenter,
		Token:          s.token,
		Service:        s.service,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, leafWatchID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for leaf cert", "error", err)
		return snap, err
	}

	// Watch the mesh config entry.
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.MeshConfig,
		Name:           structs.MeshConfigMesh,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(s.proxyID.PartitionOrDefault()),
	}, meshConfigEntryID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch the bound ai-gateway config entry. By convention the routing policy
	// shares the gateway's service name.
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.AIGateway,
		Name:           s.service,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, aiGatewayConfigWatchID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for ai-gateway config entry", "error", err)
		return snap, err
	}

	// Discover model upstreams from intentions: the set of services this gateway
	// is intention-allowed to reach. This is the same data source connect_proxy
	// uses for transparent-proxy discovery, so "catalog ∩ intentions" is baked in
	// — registering a model plus a `gateway → model = allow` intention makes it
	// discoverable with no routing-policy change.
	err = s.dataSources.IntentionUpstreams.Notify(ctx, &structs.ServiceSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		ServiceName:    s.service,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, intentionUpstreamsID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for intention upstreams", "error", err)
		return snap, err
	}

	snap.InferenceGateway.WatchedModels = make(map[structs.ServiceName]context.CancelFunc)
	snap.InferenceGateway.Models = make(map[structs.ServiceName]*InferenceGatewayModel)
	return snap, nil
}

func (s *handlerInferenceGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	switch {
	case u.CorrelationID == rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots

	case u.CorrelationID == leafWatchID:
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.InferenceGateway.Leaf = leaf

	case u.CorrelationID == meshConfigEntryID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if resp.Entry != nil {
			meshConf, ok := resp.Entry.(*structs.MeshConfigEntry)
			if !ok {
				return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
			}
			snap.InferenceGateway.MeshConfig = meshConf
		} else {
			snap.InferenceGateway.MeshConfig = nil
		}
		snap.InferenceGateway.MeshConfigSet = true

	case u.CorrelationID == aiGatewayConfigWatchID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		if resp.Entry != nil {
			cfg, ok := resp.Entry.(*structs.AIGatewayConfigEntry)
			if !ok {
				return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
			}
			snap.InferenceGateway.GatewayConfig = cfg
		} else {
			snap.InferenceGateway.GatewayConfig = nil
		}
		snap.InferenceGateway.GatewayConfigSet = true
		if err := s.reconcileStoreWatch(ctx, snap); err != nil {
			return err
		}
		return s.reconcileModelWatches(ctx, snap)

	case u.CorrelationID == inferenceStateStoreWatchID:
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		// The store is a plain Connect upstream — record its endpoints directly,
		// bypassing the ai.role model filter in updateModel.
		snap.InferenceGateway.StateStoreNodes = resp.Nodes

	case u.CorrelationID == intentionUpstreamsID:
		resp, ok := u.Result.(*structs.IndexedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.InferenceGateway.DiscoveredUpstreams = resp.Services
		return s.reconcileModelWatches(ctx, snap)

	case strings.HasPrefix(u.CorrelationID, inferenceModelServiceIDPrefix):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		sn := structs.ServiceNameFromString(strings.TrimPrefix(u.CorrelationID, inferenceModelServiceIDPrefix))
		s.updateModel(snap, sn, resp.Nodes)

	default:
		// do nothing
	}

	return nil
}

// reconcileModelWatches starts health watches for every intention-discovered
// upstream and cancels watches for any that are no longer discoverable. The
// discovery source is intention-inferred upstreams (catalog ∩ intentions), not
// the routing policy; updateModel then drops any that are not ai-model services.
func (s *handlerInferenceGateway) reconcileModelWatches(ctx context.Context, snap *ConfigSnapshot) error {
	desired := make(map[structs.ServiceName]struct{}, len(snap.InferenceGateway.DiscoveredUpstreams))
	for _, sn := range snap.InferenceGateway.DiscoveredUpstreams {
		desired[sn] = struct{}{}
	}

	// Start watches for newly referenced candidates.
	for sn := range desired {
		if _, ok := snap.InferenceGateway.WatchedModels[sn]; ok {
			continue
		}
		wctx, cancel := context.WithCancel(ctx)
		err := s.dataSources.Health.Notify(wctx, &structs.ServiceSpecificRequest{
			Datacenter:     s.source.Datacenter,
			QueryOptions:   structs.QueryOptions{Token: s.token},
			ServiceName:    sn.Name,
			EnterpriseMeta: sn.EnterpriseMeta,
			// The gateway routes to these via the terminating gateway, so we want
			// the model service's own registration (with its ai{} block + meta),
			// not connect proxies.
			Connect: false,
		}, inferenceModelServiceIDPrefix+sn.String(), s.ch)
		if err != nil {
			s.logger.Error("failed to register watch for model service",
				"service", sn.String(), "error", err)
			cancel()
			return err
		}
		snap.InferenceGateway.WatchedModels[sn] = cancel
	}

	// Cancel watches for candidates no longer referenced.
	for sn, cancel := range snap.InferenceGateway.WatchedModels {
		if _, ok := desired[sn]; !ok {
			s.logger.Debug("canceling watch for model service", "service", sn.String())
			cancel()
			delete(snap.InferenceGateway.WatchedModels, sn)
			delete(snap.InferenceGateway.Models, sn)
		}
	}

	return nil
}

// reconcileStoreWatch keeps a single Connect health watch aligned with the bound
// entry's StateStore.Service. The counter store is a Connect mesh service reached
// over mTLS (intention-gated), so — unlike the terminating-gateway model upstreams
// — we watch its sidecar-proxy endpoints (Connect: true). A changed or removed
// StateStore cancels the stale watch and clears its endpoints.
func (s *handlerInferenceGateway) reconcileStoreWatch(ctx context.Context, snap *ConfigSnapshot) error {
	ig := &snap.InferenceGateway

	var desired structs.ServiceName
	if cfg := ig.GatewayConfig; cfg != nil && cfg.StateStore != nil && cfg.StateStore.Service != "" {
		desired = structs.NewServiceName(cfg.StateStore.Service, &s.proxyID.EnterpriseMeta)
	}

	// Already watching exactly the desired store (or both are empty): nothing to do.
	if ig.StateStoreService == desired {
		return nil
	}

	// Tear down a stale watch (store removed, or the service name changed).
	if ig.StateStoreCancel != nil {
		s.logger.Debug("canceling watch for state store service", "service", ig.StateStoreService.String())
		ig.StateStoreCancel()
		ig.StateStoreCancel = nil
	}
	ig.StateStoreService = structs.ServiceName{}
	ig.StateStoreNodes = nil

	if desired.Name == "" {
		return nil
	}

	wctx, cancel := context.WithCancel(ctx)
	err := s.dataSources.Health.Notify(wctx, &structs.ServiceSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		ServiceName:    desired.Name,
		EnterpriseMeta: desired.EnterpriseMeta,
		// The counter store is a Connect mesh service reached over mTLS, so we want
		// its sidecar-proxy endpoints (intention-gated), not the bare service.
		Connect: true,
	}, inferenceStateStoreWatchID, s.ch)
	if err != nil {
		s.logger.Error("failed to register watch for state store service",
			"service", desired.String(), "error", err)
		cancel()
		return err
	}
	ig.StateStoreCancel = cancel
	ig.StateStoreService = desired
	return nil
}

// updateModel records (or clears) a discovered model upstream. A service is only
// kept if at least one instance is tagged ai.role == "ai-model"; its catalog
// Meta supplies the routing labels surfaced in the gateway listener metadata.
func (s *handlerInferenceGateway) updateModel(snap *ConfigSnapshot, sn structs.ServiceName, nodes structs.CheckServiceNodes) {
	var (
		role   string
		labels map[string]string
	)
	for _, node := range nodes {
		if node.Service == nil || node.Service.AI == nil || node.Service.AI.Role != structs.AIRoleModel {
			continue
		}
		role = node.Service.AI.Role
		labels = make(map[string]string, len(node.Service.Meta))
		for k, v := range node.Service.Meta {
			labels[k] = v
		}
		break
	}

	if role == "" {
		// Not (or no longer) an ai-model service.
		delete(snap.InferenceGateway.Models, sn)
		return
	}

	snap.InferenceGateway.Models[sn] = &InferenceGatewayModel{
		Service: sn,
		Role:    role,
		Labels:  labels,
		Nodes:   nodes,
	}
}

// candidateModelServices returns the set of model service names referenced by
// the routing policy (match-rule candidates and fallback chains, the default
// fallback chain, and any scoring split).
func candidateModelServices(e *structs.AIGatewayConfigEntry, em acl.EnterpriseMeta) map[structs.ServiceName]struct{} {
	out := make(map[structs.ServiceName]struct{})
	if e == nil {
		return out
	}
	add := func(name string) {
		if name == "" {
			return
		}
		out[structs.NewServiceName(name, &em)] = struct{}{}
	}
	for _, n := range e.Routing.FallbackChain {
		add(n)
	}
	for _, rule := range e.Routing.MatchRules {
		for _, n := range rule.Candidates {
			add(n)
		}
		for _, n := range rule.FallbackChain {
			add(n)
		}
	}
	if e.Routing.Scoring != nil {
		for _, w := range e.Routing.Scoring.WeightedSplit {
			add(w.Cluster)
		}
	}
	return out
}
