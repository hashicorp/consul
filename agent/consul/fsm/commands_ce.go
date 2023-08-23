package fsm

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/pbpeering"
)

var CommandsSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"fsm", "register"},
		Help: "Measures the time it takes to apply a catalog register operation to the FSM.",
	},
	{
		Name: []string{"fsm", "deregister"},
		Help: "Measures the time it takes to apply a catalog deregister operation to the FSM.",
	},
	{
		Name: []string{"fsm", "kvs"},
		Help: "Measures the time it takes to apply the given KV operation to the FSM.",
	},
	{
		Name: []string{"fsm", "session"},
		Help: "Measures the time it takes to apply the given session operation to the FSM.",
	},
	{
		Name: []string{"fsm", "acl"},
		Help: "Measures the time it takes to apply the given ACL operation to the FSM.",
	},
	{
		Name: []string{"fsm", "tombstone"},
		Help: "Measures the time it takes to apply the given tombstone operation to the FSM.",
	},
	{
		Name: []string{"fsm", "coordinate", "batch-update"},
		Help: "Measures the time it takes to apply the given batch coordinate update to the FSM.",
	},
	{
		Name: []string{"fsm", "prepared-query"},
		Help: "Measures the time it takes to apply the given prepared query update operation to the FSM.",
	},
	{
		Name: []string{"fsm", "txn"},
		Help: "Measures the time it takes to apply the given transaction update to the FSM.",
	},
	{
		Name: []string{"fsm", "autopilot"},
		Help: "Measures the time it takes to apply the given autopilot update to the FSM.",
	},
	{
		Name: []string{"consul", "fsm", "intention"},
		Help: "Deprecated - use fsm_intention instead",
	},
	{
		Name: []string{"fsm", "intention"},
		Help: "Measures the time it takes to apply an intention operation to the FSM.",
	},
	{
		Name: []string{"consul", "fsm", "ca"},
		Help: "Deprecated - use fsm_ca instead",
	},
	{
		Name: []string{"fsm", "ca"},
		Help: "Measures the time it takes to apply CA configuration operations to the FSM.",
	},
	{
		Name: []string{"fsm", "ca", "leaf"},
		Help: "Measures the time it takes to apply an operation while signing a leaf certificate.",
	},
	{
		Name: []string{"fsm", "acl", "token"},
		Help: "Measures the time it takes to apply an ACL token operation to the FSM.",
	},
	{
		Name: []string{"fsm", "acl", "policy"},
		Help: "Measures the time it takes to apply an ACL policy operation to the FSM.",
	},
	{
		Name: []string{"fsm", "acl", "bindingrule"},
		Help: "Measures the time it takes to apply an ACL binding rule operation to the FSM.",
	},
	{
		Name: []string{"fsm", "acl", "authmethod"},
		Help: "Measures the time it takes to apply an ACL authmethod operation to the FSM.",
	},
	{
		Name: []string{"fsm", "system_metadata"},
		Help: "Measures the time it takes to apply a system metadata operation to the FSM.",
	},
	{
		Name: []string{"fsm", "peering"},
		Help: "Measures the time it takes to apply a peering operation to the FSM.",
	},
	// TODO(kit): We generate the config-entry fsm summaries by reading off of the request. It is
	//  possible to statically declare these when we know all of the names, but I didn't get to it
	//  in this patch. Config-entries are known though and we should add these in the future.
	// {
	// 	Name:        []string{"fsm", "config_entry", req.Entry.GetKind()},
	// 	Help:        "",
	// },
}

func init() {
	registerCommand(structs.RegisterRequestType, (*FSM).applyRegister)
	registerCommand(structs.DeregisterRequestType, (*FSM).applyDeregister)
	registerCommand(structs.KVSRequestType, (*FSM).applyKVSOperation)
	registerCommand(structs.SessionRequestType, (*FSM).applySessionOperation)
	// DEPRECATED (ACL-Legacy-Compat) - Only needed for v1 ACL compat
	registerCommand(structs.DeprecatedACLRequestType, (*FSM).deprecatedApplyACLOperation)
	registerCommand(structs.TombstoneRequestType, (*FSM).applyTombstoneOperation)
	registerCommand(structs.CoordinateBatchUpdateType, (*FSM).applyCoordinateBatchUpdate)
	registerCommand(structs.PreparedQueryRequestType, (*FSM).applyPreparedQueryOperation)
	registerCommand(structs.TxnRequestType, (*FSM).applyTxn)
	registerCommand(structs.AutopilotRequestType, (*FSM).applyAutopilotUpdate)
	registerCommand(structs.IntentionRequestType, (*FSM).applyIntentionOperation)
	registerCommand(structs.ConnectCARequestType, (*FSM).applyConnectCAOperation)
	registerCommand(structs.ACLTokenSetRequestType, (*FSM).applyACLTokenSetOperation)
	registerCommand(structs.ACLTokenDeleteRequestType, (*FSM).applyACLTokenDeleteOperation)
	registerCommand(structs.ACLBootstrapRequestType, (*FSM).applyACLTokenBootstrap)
	registerCommand(structs.ACLPolicySetRequestType, (*FSM).applyACLPolicySetOperation)
	registerCommand(structs.ACLPolicyDeleteRequestType, (*FSM).applyACLPolicyDeleteOperation)
	registerCommand(structs.ConnectCALeafRequestType, (*FSM).applyConnectCALeafOperation)
	registerCommand(structs.ConfigEntryRequestType, (*FSM).applyConfigEntryOperation)
	registerCommand(structs.ACLRoleSetRequestType, (*FSM).applyACLRoleSetOperation)
	registerCommand(structs.ACLRoleDeleteRequestType, (*FSM).applyACLRoleDeleteOperation)
	registerCommand(structs.ACLBindingRuleSetRequestType, (*FSM).applyACLBindingRuleSetOperation)
	registerCommand(structs.ACLBindingRuleDeleteRequestType, (*FSM).applyACLBindingRuleDeleteOperation)
	registerCommand(structs.ACLAuthMethodSetRequestType, (*FSM).applyACLAuthMethodSetOperation)
	registerCommand(structs.ACLAuthMethodDeleteRequestType, (*FSM).applyACLAuthMethodDeleteOperation)
	registerCommand(structs.FederationStateRequestType, (*FSM).applyFederationStateOperation)
	registerCommand(structs.SystemMetadataRequestType, (*FSM).applySystemMetadataOperation)
	registerCommand(structs.PeeringWriteType, (*FSM).applyPeeringWrite)
	registerCommand(structs.PeeringDeleteType, (*FSM).applyPeeringDelete)
	registerCommand(structs.PeeringTerminateByIDType, (*FSM).applyPeeringTerminate)
	registerCommand(structs.PeeringTrustBundleWriteType, (*FSM).applyPeeringTrustBundleWrite)
	registerCommand(structs.PeeringTrustBundleDeleteType, (*FSM).applyPeeringTrustBundleDelete)
	registerCommand(structs.PeeringSecretsWriteType, (*FSM).applyPeeringSecretsWrite)
}

func (c *FSM) applyRegister(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"fsm", "register"}, time.Now())
	var req structs.RegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Apply all updates in a single transaction
	if err := c.state.EnsureRegistration(index, &req); err != nil {
		c.logger.Warn("EnsureRegistration failed", "error", err)
		return err
	}
	return nil
}

func (c *FSM) applyDeregister(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"fsm", "deregister"}, time.Now())
	var req structs.DeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Either remove the service entry or the whole node. The precedence
	// here is also baked into vetDeregisterWithACL() in acl.go, so if you
	// make changes here, be sure to also adjust the code over there.
	if req.ServiceID != "" {
		if err := c.state.DeleteService(index, req.Node, req.ServiceID, &req.EnterpriseMeta, req.PeerName); err != nil {
			c.logger.Warn("DeleteNodeService failed", "error", err)
			return err
		}
	} else if req.CheckID != "" {
		if err := c.state.DeleteCheck(index, req.Node, req.CheckID, &req.EnterpriseMeta, req.PeerName); err != nil {
			c.logger.Warn("DeleteNodeCheck failed", "error", err)
			return err
		}
	} else {
		if err := c.state.DeleteNode(index, req.Node, &req.EnterpriseMeta, req.PeerName); err != nil {
			c.logger.Warn("DeleteNode failed", "error", err)
			return err
		}
	}
	return nil
}

func (c *FSM) applyKVSOperation(buf []byte, index uint64) interface{} {
	var req structs.KVSRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "kvs"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case api.KVSet:
		return c.state.KVSSet(index, &req.DirEnt)
	case api.KVDelete:
		return c.state.KVSDelete(index, req.DirEnt.Key, &req.DirEnt.EnterpriseMeta)
	case api.KVDeleteCAS:
		act, err := c.state.KVSDeleteCAS(index, req.DirEnt.ModifyIndex, req.DirEnt.Key, &req.DirEnt.EnterpriseMeta)
		if err != nil {
			return err
		}
		return act
	case api.KVDeleteTree:
		return c.state.KVSDeleteTree(index, req.DirEnt.Key, &req.DirEnt.EnterpriseMeta)
	case api.KVCAS:
		act, err := c.state.KVSSetCAS(index, &req.DirEnt)
		if err != nil {
			return err
		}
		return act
	case api.KVLock:
		act, err := c.state.KVSLock(index, &req.DirEnt)
		if err != nil {
			return err
		}
		return act
	case api.KVUnlock:
		act, err := c.state.KVSUnlock(index, &req.DirEnt)
		if err != nil {
			return err
		}
		return act
	default:
		err := fmt.Errorf("Invalid KVS operation '%s'", req.Op)
		c.logger.Warn("Invalid KVS operation", "operation", req.Op)
		return err
	}
}

func (c *FSM) applySessionOperation(buf []byte, index uint64) interface{} {
	var req structs.SessionRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "session"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.SessionCreate:
		if err := c.state.SessionCreate(index, &req.Session); err != nil {
			return err
		}
		return req.Session.ID
	case structs.SessionDestroy:
		return c.state.SessionDestroy(index, req.Session.ID, &req.Session.EnterpriseMeta)
	default:
		c.logger.Warn("Invalid Session operation", "operation", req.Op)
		return fmt.Errorf("Invalid Session operation '%s'", req.Op)
	}
}

func (c *FSM) deprecatedApplyACLOperation(_ []byte, _ uint64) interface{} {
	return fmt.Errorf("legacy ACL command has been removed with the legacy ACL system")
}

func (c *FSM) applyTombstoneOperation(buf []byte, index uint64) interface{} {
	var req structs.TombstoneRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "tombstone"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.TombstoneReap:
		return c.state.ReapTombstones(index, req.ReapIndex)
	default:
		c.logger.Warn("Invalid Tombstone operation", "operation", req.Op)
		return fmt.Errorf("Invalid Tombstone operation '%s'", req.Op)
	}
}

// applyCoordinateBatchUpdate processes a batch of coordinate updates and applies
// them in a single underlying transaction. This interface isn't 1:1 with the outer
// update interface that the coordinate endpoint exposes, so we made it single
// purpose and avoided the opcode convention.
func (c *FSM) applyCoordinateBatchUpdate(buf []byte, index uint64) interface{} {
	var updates structs.Coordinates
	if err := structs.Decode(buf, &updates); err != nil {
		panic(fmt.Errorf("failed to decode batch updates: %v", err))
	}
	defer metrics.MeasureSince([]string{"fsm", "coordinate", "batch-update"}, time.Now())
	if err := c.state.CoordinateBatchUpdate(index, updates); err != nil {
		return err
	}
	return nil
}

// applyPreparedQueryOperation applies the given prepared query operation to the
// state store.
func (c *FSM) applyPreparedQueryOperation(buf []byte, index uint64) interface{} {
	var req structs.PreparedQueryRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "prepared-query"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.PreparedQueryCreate, structs.PreparedQueryUpdate:
		return c.state.PreparedQuerySet(index, req.Query)
	case structs.PreparedQueryDelete:
		return c.state.PreparedQueryDelete(index, req.Query.ID)
	default:
		c.logger.Warn("Invalid PreparedQuery operation", "operation", req.Op)
		return fmt.Errorf("Invalid PreparedQuery operation '%s'", req.Op)
	}
}

func (c *FSM) applyTxn(buf []byte, index uint64) interface{} {
	var req structs.TxnRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"fsm", "txn"}, time.Now())
	results, errors := c.state.TxnRW(index, req.Ops)
	return structs.TxnResponse{
		Results: results,
		Errors:  errors,
	}
}

func (c *FSM) applyAutopilotUpdate(buf []byte, index uint64) interface{} {
	var req structs.AutopilotSetConfigRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"fsm", "autopilot"}, time.Now())

	if req.CAS {
		act, err := c.state.AutopilotCASConfig(index, req.Config.ModifyIndex, &req.Config)
		if err != nil {
			return err
		}
		return act
	}
	return c.state.AutopilotSetConfig(index, &req.Config)
}

// applyIntentionOperation applies the given intention operation to the state store.
func (c *FSM) applyIntentionOperation(buf []byte, index uint64) interface{} {
	var req structs.IntentionRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// TODO(kit): We should deprecate this first metric that writes the metrics_prefix itself,
	//  the config we use to flag this out, telemetry.disable_compat_1.9 is on the agent - how do
	//  we access it here?
	defer metrics.MeasureSinceWithLabels([]string{"consul", "fsm", "intention"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "intention"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})

	if req.Mutation != nil {
		return c.state.IntentionMutation(index, req.Op, req.Mutation)
	}

	switch req.Op {
	case structs.IntentionOpCreate, structs.IntentionOpUpdate:
		//nolint:staticcheck
		return c.state.LegacyIntentionSet(index, req.Intention)
	case structs.IntentionOpDelete:
		//nolint:staticcheck
		return c.state.LegacyIntentionDelete(index, req.Intention.ID)
	case structs.IntentionOpDeleteAll:
		return c.state.LegacyIntentionDeleteAll(index)
	case structs.IntentionOpUpsert:
		fallthrough // unsupported
	default:
		c.logger.Warn("Invalid Intention operation", "operation", req.Op)
		return fmt.Errorf("Invalid Intention operation '%s'", req.Op)
	}
}

// applyConnectCAOperation applies the given CA operation to the state store.
func (c *FSM) applyConnectCAOperation(buf []byte, index uint64) interface{} {
	var req structs.CARequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"consul", "fsm", "ca"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "ca"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})

	result := ApplyConnectCAOperationFromRequest(c.state, &req, index)
	if err, ok := result.(error); ok && err != nil {
		c.logger.Warn("Failed to apply CA operation", "operation", req.Op)
	}
	return result
}

func ApplyConnectCAOperationFromRequest(state *state.Store, req *structs.CARequest, index uint64) interface{} {
	switch req.Op {
	case structs.CAOpSetConfig:
		if req.Config.ModifyIndex != 0 {
			act, err := state.CACheckAndSetConfig(index, req.Config.ModifyIndex, req.Config)
			if err != nil {
				return err
			}

			return act
		}

		return state.CASetConfig(index, req.Config)
	case structs.CAOpSetRoots:
		act, err := state.CARootSetCAS(index, req.Index, req.Roots)
		if err != nil {
			return err
		}

		return act
	case structs.CAOpSetProviderState:
		act, err := state.CASetProviderState(index, req.ProviderState)
		if err != nil {
			return err
		}

		return act
	case structs.CAOpDeleteProviderState:
		if err := state.CADeleteProviderState(index, req.ProviderState.ID); err != nil {
			return err
		}

		return true
	case structs.CAOpSetRootsAndConfig:
		act, err := state.CARootSetCAS(index, req.Index, req.Roots)
		if err != nil {
			return err
		}
		if !act {
			return act
		}

		act, err = state.CACheckAndSetConfig(index, req.Config.ModifyIndex, req.Config)
		if err != nil {
			return err
		}
		return act
	case structs.CAOpIncrementProviderSerialNumber:
		sn, err := state.CAIncrementProviderSerialNumber(index)
		if err != nil {
			return err
		}

		return sn
	default:
		return fmt.Errorf("Invalid CA operation '%s'", req.Op)
	}
}

// applyConnectCALeafOperation applies an operation while signing a leaf certificate.
func (c *FSM) applyConnectCALeafOperation(buf []byte, index uint64) interface{} {
	var req structs.CALeafRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "ca", "leaf"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.CALeafOpIncrementIndex:
		// Use current index as the new value as well as the value to write at.
		if err := c.state.CALeafSetIndex(index, index); err != nil {
			return err
		}
		return index
	default:
		c.logger.Warn("Invalid CA Leaf operation", "operation", req.Op)
		return fmt.Errorf("Invalid CA operation '%s'", req.Op)
	}
}

func (c *FSM) applyACLTokenSetOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLTokenBatchSetRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "token"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "upsert"}})

	opts := state.ACLTokenSetOptions{
		CAS:                          req.CAS,
		AllowMissingPolicyAndRoleIDs: req.AllowMissingLinks,
		ProhibitUnprivileged:         req.ProhibitUnprivileged,
		FromReplication:              req.FromReplication,
	}
	return c.state.ACLTokenBatchSet(index, req.Tokens, opts)
}

func (c *FSM) applyACLTokenDeleteOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLTokenBatchDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "token"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	return c.state.ACLTokenBatchDelete(index, req.TokenIDs)
}

func (c *FSM) applyACLTokenBootstrap(buf []byte, index uint64) interface{} {
	var req structs.ACLTokenBootstrapRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "token"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "bootstrap"}})
	return c.state.ACLBootstrap(index, req.ResetIndex, &req.Token)
}

func (c *FSM) applyACLPolicySetOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLPolicyBatchSetRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "policy"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "upsert"}})

	return c.state.ACLPolicyBatchSet(index, req.Policies)
}

func (c *FSM) applyACLPolicyDeleteOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLPolicyBatchDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "policy"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	return c.state.ACLPolicyBatchDelete(index, req.PolicyIDs)
}

func (c *FSM) applyConfigEntryOperation(buf []byte, index uint64) interface{} {
	req := structs.ConfigEntryRequest{
		Entry: &structs.ProxyConfigEntry{},
	}
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	switch req.Op {
	case structs.ConfigEntryUpsertCAS:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "config_entry", req.Entry.GetKind()}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "upsert"}})
		updated, err := c.state.EnsureConfigEntryCAS(index, req.Entry.GetRaftIndex().ModifyIndex, req.Entry)
		if err != nil {
			return err
		}
		return updated
	case structs.ConfigEntryUpsert:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "config_entry", req.Entry.GetKind()}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "upsert"}})
		if err := c.state.EnsureConfigEntry(index, req.Entry); err != nil {
			return err
		}
		return true
	case structs.ConfigEntryDeleteCAS:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "config_entry", req.Entry.GetKind()}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "delete"}})
		deleted, err := c.state.DeleteConfigEntryCAS(index, req.Entry.GetRaftIndex().ModifyIndex, req.Entry)
		if err != nil {
			return err
		}
		return deleted
	case structs.ConfigEntryDelete:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "config_entry", req.Entry.GetKind()}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "delete"}})
		return c.state.DeleteConfigEntry(index, req.Entry.GetKind(), req.Entry.GetName(), req.Entry.GetEnterpriseMeta())
	default:
		return fmt.Errorf("invalid config entry operation type: %v", req.Op)
	}
}

func (c *FSM) applyACLRoleSetOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLRoleBatchSetRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "role"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "upsert"}})

	return c.state.ACLRoleBatchSet(index, req.Roles, req.AllowMissingLinks)
}

func (c *FSM) applyACLRoleDeleteOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLRoleBatchDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "role"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	return c.state.ACLRoleBatchDelete(index, req.RoleIDs)
}

func (c *FSM) applyACLBindingRuleSetOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLBindingRuleBatchSetRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "bindingrule"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "upsert"}})

	return c.state.ACLBindingRuleBatchSet(index, req.BindingRules)
}

func (c *FSM) applyACLBindingRuleDeleteOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLBindingRuleBatchDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "bindingrule"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	return c.state.ACLBindingRuleBatchDelete(index, req.BindingRuleIDs)
}

func (c *FSM) applyACLAuthMethodSetOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLAuthMethodBatchSetRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "authmethod"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "upsert"}})

	return c.state.ACLAuthMethodBatchSet(index, req.AuthMethods)
}

func (c *FSM) applyACLAuthMethodDeleteOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLAuthMethodBatchDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl", "authmethod"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	return c.state.ACLAuthMethodBatchDelete(index, req.AuthMethodNames, &req.EnterpriseMeta)
}

func (c *FSM) applyFederationStateOperation(buf []byte, index uint64) interface{} {
	var req structs.FederationStateRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	switch req.Op {
	case structs.FederationStateUpsert:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "federation_state", req.State.Datacenter}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "upsert"}})
		if err := c.state.FederationStateSet(index, req.State); err != nil {
			return err
		}
		return true
	case structs.FederationStateDelete:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "federation_state", req.State.Datacenter}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "delete"}})
		return c.state.FederationStateDelete(index, req.State.Datacenter)
	default:
		return fmt.Errorf("invalid federation state operation type: %v", req.Op)
	}
}

func (c *FSM) applySystemMetadataOperation(buf []byte, index uint64) interface{} {
	var req structs.SystemMetadataRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	switch req.Op {
	case structs.SystemMetadataUpsert:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "system_metadata"}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "upsert"}})
		if err := c.state.SystemMetadataSet(index, req.Entry); err != nil {
			return err
		}
		return true
	case structs.SystemMetadataDelete:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "system_metadata"}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "delete"}})
		return c.state.SystemMetadataDelete(index, req.Entry)
	default:
		return fmt.Errorf("invalid system metadata operation type: %v", req.Op)
	}
}

func (c *FSM) applyPeeringWrite(buf []byte, index uint64) interface{} {
	var req pbpeering.PeeringWriteRequest
	if err := structs.DecodeProto(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode peering write request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "peering"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "write"}})

	return c.state.PeeringWrite(index, &req)
}

func (c *FSM) applyPeeringDelete(buf []byte, index uint64) interface{} {
	var req pbpeering.PeeringDeleteRequest
	if err := structs.DecodeProto(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode peering delete request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "peering"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	q := state.Query{
		Value:          req.Name,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
	}
	return c.state.PeeringDelete(index, q)
}

func (c *FSM) applyPeeringSecretsWrite(buf []byte, index uint64) interface{} {
	var req pbpeering.SecretsWriteRequest
	if err := structs.DecodeProto(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode peering secrets write request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "peering_secrets"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "write"}})

	return c.state.PeeringSecretsWrite(index, &req)
}

func (c *FSM) applyPeeringTerminate(buf []byte, index uint64) interface{} {
	var req pbpeering.PeeringTerminateByIDRequest
	if err := structs.DecodeProto(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode peering delete request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "peering"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "terminate"}})

	return c.state.PeeringTerminateByID(index, req.ID)
}

func (c *FSM) applyPeeringTrustBundleWrite(buf []byte, index uint64) interface{} {
	var req pbpeering.PeeringTrustBundleWriteRequest
	if err := structs.DecodeProto(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode peering trust bundle write request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "peering_trust_bundle"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "write"}})

	return c.state.PeeringTrustBundleWrite(index, req.PeeringTrustBundle)
}

func (c *FSM) applyPeeringTrustBundleDelete(buf []byte, index uint64) interface{} {
	var req pbpeering.PeeringTrustBundleDeleteRequest
	if err := structs.DecodeProto(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode peering trust bundle delete request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "peering_trust_bundle"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: "delete"}})

	q := state.Query{
		Value:          req.Name,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Partition),
	}
	return c.state.PeeringTrustBundleDelete(index, q)
}
