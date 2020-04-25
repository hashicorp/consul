package fsm

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func init() {
	registerCommand(structs.RegisterRequestType, (*FSM).applyRegister)
	registerCommand(structs.DeregisterRequestType, (*FSM).applyDeregister)
	registerCommand(structs.KVSRequestType, (*FSM).applyKVSOperation)
	registerCommand(structs.SessionRequestType, (*FSM).applySessionOperation)
	// DEPRECATED (ACL-Legacy-Compat) - Only needed for v1 ACL compat
	registerCommand(structs.ACLRequestType, (*FSM).applyACLOperation)
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
		if err := c.state.DeleteService(index, req.Node, req.ServiceID, &req.EnterpriseMeta); err != nil {
			c.logger.Warn("DeleteNodeService failed", "error", err)
			return err
		}
	} else if req.CheckID != "" {
		if err := c.state.DeleteCheck(index, req.Node, req.CheckID, &req.EnterpriseMeta); err != nil {
			c.logger.Warn("DeleteNodeCheck failed", "error", err)
			return err
		}
	} else {
		if err := c.state.DeleteNode(index, req.Node); err != nil {
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

// DEPRECATED (ACL-Legacy-Compat) - Only needed for legacy compat
func (c *FSM) applyACLOperation(buf []byte, index uint64) interface{} {
	// TODO (ACL-Legacy-Compat) - Should we warn here somehow about using deprecated features
	//                            maybe emit a second metric?
	var req structs.ACLRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.ACLBootstrapInit:
		enabled, _, err := c.state.CanBootstrapACLToken()
		if err != nil {
			return err
		}
		return enabled
	case structs.ACLBootstrapNow:
		// This is a bootstrap request from a non-upgraded node
		if err := c.state.ACLBootstrap(index, 0, req.ACL.Convert(), true); err != nil {
			return err
		}

		// No need to check expiration times as those did not exist in legacy tokens.
		if _, token, err := c.state.ACLTokenGetBySecret(nil, req.ACL.ID, nil); err != nil {
			return err
		} else {
			acl, err := token.Convert()
			if err != nil {
				return err
			}
			return acl
		}

	case structs.ACLForceSet, structs.ACLSet:
		if err := c.state.ACLTokenSet(index, req.ACL.Convert(), true); err != nil {
			return err
		}
		return req.ACL.ID
	case structs.ACLDelete:
		return c.state.ACLTokenDeleteBySecret(index, req.ACL.ID, nil)
	default:
		c.logger.Warn("Invalid ACL operation", "operation", req.Op)
		return fmt.Errorf("Invalid ACL operation '%s'", req.Op)
	}
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
		return c.state.ReapTombstones(req.ReapIndex)
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

	defer metrics.MeasureSinceWithLabels([]string{"consul", "fsm", "intention"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "intention"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.IntentionOpCreate, structs.IntentionOpUpdate:
		return c.state.IntentionSet(index, req.Intention)
	case structs.IntentionOpDelete:
		return c.state.IntentionDelete(index, req.Intention.ID)
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
	switch req.Op {
	case structs.CAOpSetConfig:
		if req.Config.ModifyIndex != 0 {
			act, err := c.state.CACheckAndSetConfig(index, req.Config.ModifyIndex, req.Config)
			if err != nil {
				return err
			}

			return act
		}

		return c.state.CASetConfig(index, req.Config)
	case structs.CAOpSetRoots:
		act, err := c.state.CARootSetCAS(index, req.Index, req.Roots)
		if err != nil {
			return err
		}

		return act
	case structs.CAOpSetProviderState:
		act, err := c.state.CASetProviderState(index, req.ProviderState)
		if err != nil {
			return err
		}

		return act
	case structs.CAOpDeleteProviderState:
		if err := c.state.CADeleteProviderState(req.ProviderState.ID); err != nil {
			return err
		}

		return true
	case structs.CAOpSetRootsAndConfig:
		act, err := c.state.CARootSetCAS(index, req.Index, req.Roots)
		if err != nil {
			return err
		}
		if !act {
			return act
		}

		act, err = c.state.CACheckAndSetConfig(index+1, req.Config.ModifyIndex, req.Config)
		if err != nil {
			return err
		}
		return act
	case structs.CAOpIncrementProviderSerialNumber:
		sn, err := c.state.CAIncrementProviderSerialNumber()
		if err != nil {
			return err
		}

		return sn
	default:
		c.logger.Warn("Invalid CA operation", "operation", req.Op)
		return fmt.Errorf("Invalid CA operation '%s'", req.Op)
	}
}

func (c *FSM) applyConnectCALeafOperation(buf []byte, index uint64) interface{} {
	var req structs.CALeafRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	defer metrics.MeasureSinceWithLabels([]string{"fsm", "ca", "leaf"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.CALeafOpIncrementIndex:
		if err := c.state.CALeafSetIndex(index); err != nil {
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

	return c.state.ACLTokenBatchSet(index, req.Tokens, req.CAS, req.AllowMissingLinks, req.ProhibitUnprivileged)
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
	return c.state.ACLBootstrap(index, req.ResetIndex, &req.Token, false)
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
		updated, err := c.state.EnsureConfigEntryCAS(index, req.Entry.GetRaftIndex().ModifyIndex, req.Entry, req.Entry.GetEnterpriseMeta())
		if err != nil {
			return err
		}
		return updated
	case structs.ConfigEntryUpsert:
		defer metrics.MeasureSinceWithLabels([]string{"fsm", "config_entry", req.Entry.GetKind()}, time.Now(),
			[]metrics.Label{{Name: "op", Value: "upsert"}})
		if err := c.state.EnsureConfigEntry(index, req.Entry, req.Entry.GetEnterpriseMeta()); err != nil {
			return err
		}
		return true
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
