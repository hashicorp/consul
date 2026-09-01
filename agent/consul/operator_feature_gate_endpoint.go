// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/featuregate"
	"github.com/hashicorp/consul/agent/structs"
)

func (op *Operator) FeatureGateGet(args *structs.FeatureGateQueryRequest, reply *structs.FeatureGateQueryResponse) error {
	if done, err := op.srv.ForwardRPC("Operator.FeatureGateGet", args, reply); done {
		return err
	}

	authz, err := op.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(authz.Identity()); err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorReadAllowed(nil); err != nil {
		return err
	}

	return op.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta, func(ws memdb.WatchSet, stateStore *state.Store) error {
		index, policy, status, err := stateStore.FeatureGatePolicyAndStatus(ws)
		if err != nil {
			return err
		}
		// Policy or status not yet initialized — return a well-formed empty
		// response instead of an error.  Blocking queries will wake up once the
		// leader commits the first policy/status generation.
		if policy == nil || status == nil {
			reply.Index = index
			reply.Features = []structs.FeatureGateInfo{}
			reply.Uninitialized = true
			return nil
		}
		features, err := featureGateInfos(op.srv.featureGateRegistry, policy, status, args.Name)
		if err != nil {
			return err
		}
		reply.Index = index
		reply.Features = features
		return nil
	})
}

func (op *Operator) FeatureGateSet(args *structs.FeatureGateSetRequest, reply *structs.FeatureGateSetResponse) error {
	if done, err := op.srv.ForwardRPC("Operator.FeatureGateSet", args, reply); done {
		return err
	}

	authz, err := op.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(authz.Identity()); err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	if _, ok := op.srv.featureGateRegistry.DefinitionForName(args.Name); !ok {
		return fmt.Errorf("unknown feature gate %q", args.Name)
	}

	_, policy, status, err := op.srv.fsm.State().FeatureGatePolicyAndStatus(nil)
	if err != nil {
		return err
	}
	if policy == nil || status == nil {
		return fmt.Errorf("feature-gate policy is not initialized yet")
	}
	if args.ExpectedPolicyIndex != 0 && args.ExpectedPolicyIndex != policy.ModifyIndex {
		return op.populateFeatureGateSetResponse(reply, false, args.Name, policy, status)
	}

	current, exists := policy.Settings[args.Name]
	if exists && current.Enabled == args.Enabled && current.Source == structs.FeatureGateSourceOperator {
		return op.populateFeatureGateSetResponse(reply, true, args.Name, policy, status)
	}

	nextPolicy := policy.Clone()
	if nextPolicy.Settings == nil {
		nextPolicy.Settings = make(map[string]structs.FeatureGateSetting)
	}
	nextPolicy.Settings[args.Name] = structs.FeatureGateSetting{
		Enabled: args.Enabled,
		Source:  structs.FeatureGateSourceOperator,
	}
	// Use the same server-version resolver as the leader loop so operator writes
	// atomically commit intent and its final resolved status.
	nextStatus := op.srv.resolveFeatureGateStatus(nextPolicy)
	request := &structs.FeatureGateUpdateRequest{
		Policy:              nextPolicy,
		Status:              nextStatus,
		ExpectedPolicyIndex: policy.ModifyIndex,
		ExpectedStatusIndex: status.ModifyIndex,
	}
	// See reconcileFeatureGates: this state can be ignored by binaries that
	// predate the framework, including when they replay an older log entry
	// during a downgrade.
	response, err := op.srv.raftApplyMsgpack(
		structs.FeatureGateRequestType|structs.IgnoreUnknownTypeFlag,
		request,
	)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}
	applied, ok := response.(bool)
	if !ok {
		return fmt.Errorf("feature-gate update returned unexpected response %T", response)
	}

	_, committedPolicy, committedStatus, err := op.srv.fsm.State().FeatureGatePolicyAndStatus(nil)
	if err != nil {
		return err
	}
	return op.populateFeatureGateSetResponse(reply, applied, args.Name, committedPolicy, committedStatus)
}

func (s *Server) resolveFeatureGateStatus(policy *structs.FeatureGatePolicy) *structs.FeatureGateStatus {
	return resolveFeatureGateStatus(s.featureGateRegistry, policy, func(minimum *version.Version) (bool, bool) {
		return ServersInDCMeetMinimumVersion(s, s.config.Datacenter, minimum)
	})
}

func (op *Operator) populateFeatureGateSetResponse(reply *structs.FeatureGateSetResponse, applied bool, name string, policy *structs.FeatureGatePolicy, status *structs.FeatureGateStatus) error {
	features, err := featureGateInfos(op.srv.featureGateRegistry, policy, status, name)
	if err != nil {
		return err
	}
	if len(features) == 0 {
		return fmt.Errorf("feature gate response was empty")
	}
	reply.Applied = applied
	reply.Feature = features[0]
	return nil
}

func featureGateInfos(registry featuregate.Registry, policy *structs.FeatureGatePolicy, status *structs.FeatureGateStatus, name string) ([]structs.FeatureGateInfo, error) {
	if name != "" {
		definition, ok := registry.DefinitionForName(name)
		if !ok {
			return nil, fmt.Errorf("unknown feature gate %q", name)
		}
		info, err := featureGateInfo(definition, policy, status)
		if err != nil {
			return nil, err
		}
		return []structs.FeatureGateInfo{info}, nil
	}

	definitions := registry.Definitions()
	features := make([]structs.FeatureGateInfo, 0, len(definitions))
	for _, definition := range definitions {
		info, err := featureGateInfo(definition, policy, status)
		if err != nil {
			return nil, err
		}
		features = append(features, info)
	}
	return features, nil
}

func featureGateInfo(definition featuregate.Definition, policy *structs.FeatureGatePolicy, status *structs.FeatureGateStatus) (structs.FeatureGateInfo, error) {
	resolved, ok := status.Features[definition.Name]
	if !ok {
		return structs.FeatureGateInfo{}, fmt.Errorf("feature gate %q is missing from committed status", definition.Name)
	}
	return structs.FeatureGateInfo{
		Name:                 definition.Name,
		Stage:                string(definition.Stage),
		MinVersion:           definition.MinVersion.String(),
		DefaultEnabled:       definition.DefaultEnabled,
		BeforeMinimumVersion: string(definition.BeforeMinimumVersion),
		Description:          definition.Description,
		Owner:                definition.Owner,
		DesiredEnabled:       resolved.DesiredEnabled,
		EffectiveEnabled:     resolved.EffectiveEnabled,
		Eligible:             resolved.Eligible,
		Source:               resolved.Source,
		Reason:               resolved.Reason,
		PolicyIndex:          policy.ModifyIndex,
		StatusIndex:          status.ModifyIndex,
	}, nil
}
