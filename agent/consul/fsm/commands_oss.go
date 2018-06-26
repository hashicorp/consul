package fsm

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func init() {
	registerCommand(structs.RegisterRequestType, (*FSM).applyRegister)
	registerCommand(structs.DeregisterRequestType, (*FSM).applyDeregister)
	registerCommand(structs.KVSRequestType, (*FSM).applyKVSOperation)
	registerCommand(structs.SessionRequestType, (*FSM).applySessionOperation)
	registerCommand(structs.ACLRequestType, (*FSM).applyACLOperation)
	registerCommand(structs.TombstoneRequestType, (*FSM).applyTombstoneOperation)
	registerCommand(structs.CoordinateBatchUpdateType, (*FSM).applyCoordinateBatchUpdate)
	registerCommand(structs.PreparedQueryRequestType, (*FSM).applyPreparedQueryOperation)
	registerCommand(structs.TxnRequestType, (*FSM).applyTxn)
	registerCommand(structs.AutopilotRequestType, (*FSM).applyAutopilotUpdate)
	registerCommand(structs.IntentionRequestType, (*FSM).applyIntentionOperation)
	registerCommand(structs.ConnectCARequestType, (*FSM).applyConnectCAOperation)
}

func (c *FSM) applyRegister(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"fsm", "register"}, time.Now())
	var req structs.RegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Apply all updates in a single transaction
	if err := c.state.EnsureRegistration(index, &req); err != nil {
		c.logger.Printf("[WARN] consul.fsm: EnsureRegistration failed: %v", err)
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
		if err := c.state.DeleteService(index, req.Node, req.ServiceID); err != nil {
			c.logger.Printf("[WARN] consul.fsm: DeleteNodeService failed: %v", err)
			return err
		}
	} else if req.CheckID != "" {
		if err := c.state.DeleteCheck(index, req.Node, req.CheckID); err != nil {
			c.logger.Printf("[WARN] consul.fsm: DeleteNodeCheck failed: %v", err)
			return err
		}
	} else {
		if err := c.state.DeleteNode(index, req.Node); err != nil {
			c.logger.Printf("[WARN] consul.fsm: DeleteNode failed: %v", err)
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
		return c.state.KVSDelete(index, req.DirEnt.Key)
	case api.KVDeleteCAS:
		act, err := c.state.KVSDeleteCAS(index, req.DirEnt.ModifyIndex, req.DirEnt.Key)
		if err != nil {
			return err
		}
		return act
	case api.KVDeleteTree:
		return c.state.KVSDeleteTree(index, req.DirEnt.Key)
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
		c.logger.Printf("[WARN] consul.fsm: %v", err)
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
		return c.state.SessionDestroy(index, req.Session.ID)
	default:
		c.logger.Printf("[WARN] consul.fsm: Invalid Session operation '%s'", req.Op)
		return fmt.Errorf("Invalid Session operation '%s'", req.Op)
	}
}

func (c *FSM) applyACLOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSinceWithLabels([]string{"fsm", "acl"}, time.Now(),
		[]metrics.Label{{Name: "op", Value: string(req.Op)}})
	switch req.Op {
	case structs.ACLBootstrapInit:
		enabled, err := c.state.ACLBootstrapInit(index)
		if err != nil {
			return err
		}
		return enabled
	case structs.ACLBootstrapNow:
		if err := c.state.ACLBootstrap(index, &req.ACL); err != nil {
			return err
		}
		return &req.ACL
	case structs.ACLForceSet, structs.ACLSet:
		if err := c.state.ACLSet(index, &req.ACL); err != nil {
			return err
		}
		return req.ACL.ID
	case structs.ACLDelete:
		return c.state.ACLDelete(index, req.ACL.ID)
	default:
		c.logger.Printf("[WARN] consul.fsm: Invalid ACL operation '%s'", req.Op)
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
		c.logger.Printf("[WARN] consul.fsm: Invalid Tombstone operation '%s'", req.Op)
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
		c.logger.Printf("[WARN] consul.fsm: Invalid PreparedQuery operation '%s'", req.Op)
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
		c.logger.Printf("[WARN] consul.fsm: Invalid Intention operation '%s'", req.Op)
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

		if err := c.state.CASetConfig(index+1, req.Config); err != nil {
			return err
		}

		return act
	default:
		c.logger.Printf("[WARN] consul.fsm: Invalid CA operation '%s'", req.Op)
		return fmt.Errorf("Invalid CA operation '%s'", req.Op)
	}
}
