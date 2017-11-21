package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sentinel"
	"github.com/hashicorp/go-memdb"
)

// KVS endpoint is used to manipulate the Key-Value store
type KVS struct {
	srv *Server
}

// preApply does all the verification of a KVS update that is performed BEFORE
// we submit as a Raft log entry. This includes enforcing the lock delay which
// must only be done on the leader.
func kvsPreApply(srv *Server, rule acl.ACL, op api.KVOp, dirEnt *structs.DirEntry) (bool, error) {
	// Verify the entry.

	if dirEnt.Key == "" && op != api.KVDeleteTree {
		return false, fmt.Errorf("Must provide key")
	}

	// Apply the ACL policy if any.
	if rule != nil {
		switch op {
		case api.KVDeleteTree:
			if !rule.KeyWritePrefix(dirEnt.Key) {
				return false, acl.ErrPermissionDenied
			}

		case api.KVGet, api.KVGetTree:
			// Filtering for GETs is done on the output side.

		case api.KVCheckSession, api.KVCheckIndex:
			// These could reveal information based on the outcome
			// of the transaction, and they operate on individual
			// keys so we check them here.
			if !rule.KeyRead(dirEnt.Key) {
				return false, acl.ErrPermissionDenied
			}

		default:
			scope := func() map[string]interface{} {
				return sentinel.ScopeKVUpsert(dirEnt.Key, dirEnt.Value, dirEnt.Flags)
			}
			if !rule.KeyWrite(dirEnt.Key, scope) {
				return false, acl.ErrPermissionDenied
			}
		}
	}

	// If this is a lock, we must check for a lock-delay. Since lock-delay
	// is based on wall-time, each peer would expire the lock-delay at a slightly
	// different time. This means the enforcement of lock-delay cannot be done
	// after the raft log is committed as it would lead to inconsistent FSMs.
	// Instead, the lock-delay must be enforced before commit. This means that
	// only the wall-time of the leader node is used, preventing any inconsistencies.
	if op == api.KVLock {
		state := srv.fsm.State()
		expires := state.KVSLockDelay(dirEnt.Key)
		if expires.After(time.Now()) {
			srv.logger.Printf("[WARN] consul.kvs: Rejecting lock of %s due to lock-delay until %v",
				dirEnt.Key, expires)
			return false, nil
		}
	}

	return true, nil
}

// Apply is used to apply a KVS update request to the data store.
func (k *KVS) Apply(args *structs.KVSRequest, reply *bool) error {
	if done, err := k.srv.forward("KVS.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "kvs", "apply"}, time.Now())
	defer metrics.MeasureSince([]string{"kvs", "apply"}, time.Now())

	// Perform the pre-apply checks.
	acl, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	ok, err := kvsPreApply(k.srv, acl, args.Op, &args.DirEnt)
	if err != nil {
		return err
	}
	if !ok {
		*reply = false
		return nil
	}

	// Apply the update.
	resp, err := k.srv.raftApply(structs.KVSRequestType, args)
	if err != nil {
		k.srv.logger.Printf("[ERR] consul.kvs: Apply failed: %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Check if the return type is a bool.
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}
	return nil
}

// Get is used to lookup a single key.
func (k *KVS) Get(args *structs.KeyRequest, reply *structs.IndexedDirEntries) error {
	if done, err := k.srv.forward("KVS.Get", args, args, reply); done {
		return err
	}

	aclRule, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	return k.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, ent, err := state.KVSGet(ws, args.Key)
			if err != nil {
				return err
			}
			if aclRule != nil && !aclRule.KeyRead(args.Key) {
				return acl.ErrPermissionDenied
			}

			if ent == nil {
				// Must provide non-zero index to prevent blocking
				// Index 1 is impossible anyways (due to Raft internals)
				if index == 0 {
					reply.Index = 1
				} else {
					reply.Index = index
				}
				reply.Entries = nil
			} else {
				reply.Index = ent.ModifyIndex
				reply.Entries = structs.DirEntries{ent}
			}
			return nil
		})
}

// List is used to list all keys with a given prefix.
func (k *KVS) List(args *structs.KeyRequest, reply *structs.IndexedDirEntries) error {
	if done, err := k.srv.forward("KVS.List", args, args, reply); done {
		return err
	}

	aclToken, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	if aclToken != nil && k.srv.config.ACLEnableKeyListPolicy && !aclToken.KeyList(args.Key) {
		return acl.ErrPermissionDenied
	}

	return k.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, ent, err := state.KVSList(ws, args.Key)
			if err != nil {
				return err
			}
			if aclToken != nil {
				ent = FilterDirEnt(aclToken, ent)
			}

			if len(ent) == 0 {
				// Must provide non-zero index to prevent blocking
				// Index 1 is impossible anyways (due to Raft internals)
				if index == 0 {
					reply.Index = 1
				} else {
					reply.Index = index
				}
				reply.Entries = nil
			} else {
				reply.Index = index
				reply.Entries = ent
			}
			return nil
		})
}

// ListKeys is used to list all keys with a given prefix to a separator.
func (k *KVS) ListKeys(args *structs.KeyListRequest, reply *structs.IndexedKeyList) error {
	if done, err := k.srv.forward("KVS.ListKeys", args, args, reply); done {
		return err
	}

	aclToken, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	if aclToken != nil && k.srv.config.ACLEnableKeyListPolicy && !aclToken.KeyList(args.Prefix) {
		return acl.ErrPermissionDenied
	}

	return k.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, keys, err := state.KVSListKeys(ws, args.Prefix, args.Seperator)
			if err != nil {
				return err
			}

			// Must provide non-zero index to prevent blocking
			// Index 1 is impossible anyways (due to Raft internals)
			if index == 0 {
				reply.Index = 1
			} else {
				reply.Index = index
			}

			if aclToken != nil {
				keys = FilterKeys(aclToken, keys)
			}
			reply.Keys = keys
			return nil
		})
}
