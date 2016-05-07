package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/consul/structs"
)

// KVS endpoint is used to manipulate the Key-Value store
type KVS struct {
	srv *Server
}

// preApply does all the verification of a KVS update that is performed BEFORE
// we submit as a Raft log entry. This includes enforcing the lock delay which
// must only be done on the leader.
func (k *KVS) preApply(acl acl.ACL, op structs.KVSOp, dirEnt *structs.DirEntry) (bool, error) {
	// Verify the entry.
	if dirEnt.Key == "" && op != structs.KVSDeleteTree {
		return false, fmt.Errorf("Must provide key")
	}

	// Apply the ACL policy if any.
	if acl != nil {
		switch op {
		case structs.KVSDeleteTree:
			if !acl.KeyWritePrefix(dirEnt.Key) {
				return false, permissionDeniedErr
			}
		default:
			if !acl.KeyWrite(dirEnt.Key) {
				return false, permissionDeniedErr
			}
		}
	}

	// If this is a lock, we must check for a lock-delay. Since lock-delay
	// is based on wall-time, each peer would expire the lock-delay at a slightly
	// different time. This means the enforcement of lock-delay cannot be done
	// after the raft log is committed as it would lead to inconsistent FSMs.
	// Instead, the lock-delay must be enforced before commit. This means that
	// only the wall-time of the leader node is used, preventing any inconsistencies.
	if op == structs.KVSLock {
		state := k.srv.fsm.State()
		expires := state.KVSLockDelay(dirEnt.Key)
		if expires.After(time.Now()) {
			k.srv.logger.Printf("[WARN] consul.kvs: Rejecting lock of %s due to lock-delay until %v",
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

	// Perform the pre-apply checks.
	acl, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	ok, err := k.preApply(acl, args.Op, &args.DirEnt)
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

// AtomicApply is used to apply multiple KVS operations in a single, atomic
// transaction.
func (k *KVS) AtomicApply(args *structs.KVSAtomicRequest, reply *structs.KVSAtomicResponse) error {
	if done, err := k.srv.forward("KVS.AtomicApply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "kvs", "apply-atomic"}, time.Now())

	// Perform the pre-apply checks on each of the operations.
	acl, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	for i, op := range args.Ops {
		ok, err := k.preApply(acl, op.Op, &op.DirEnt)
		if err != nil {
			reply.Errors = append(reply.Errors, &structs.KVSAtomicError{i, err.Error()})
		} else if !ok {
			err = fmt.Errorf("failed to lock key %q due to lock delay", op.DirEnt.Key)
			reply.Errors = append(reply.Errors, &structs.KVSAtomicError{i, err.Error()})
		}
	}
	if len(reply.Errors) > 0 {
		return nil
	}

	// Apply the update.
	resp, err := k.srv.raftApply(structs.KVSAtomicRequestType, args)
	if err != nil {
		k.srv.logger.Printf("[ERR] consul.kvs: ApplyAtomic failed: %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Convert the return type. This should be a cheap copy since we are
	// just taking the two slices.
	if respAtomic, ok := resp.(structs.KVSAtomicResponse); ok {
		*reply = respAtomic
	} else {
		return fmt.Errorf("unexpected return type %T", resp)
	}
	return nil
}

// Get is used to lookup a single key.
func (k *KVS) Get(args *structs.KeyRequest, reply *structs.IndexedDirEntries) error {
	if done, err := k.srv.forward("KVS.Get", args, args, reply); done {
		return err
	}

	acl, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	// Get the local state
	state := k.srv.fsm.State()
	return k.srv.blockingRPC(
		&args.QueryOptions,
		&reply.QueryMeta,
		state.GetKVSWatch(args.Key),
		func() error {
			index, ent, err := state.KVSGet(args.Key)
			if err != nil {
				return err
			}
			if acl != nil && !acl.KeyRead(args.Key) {
				ent = nil
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

	acl, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	// Get the local state
	state := k.srv.fsm.State()
	return k.srv.blockingRPC(
		&args.QueryOptions,
		&reply.QueryMeta,
		state.GetKVSWatch(args.Key),
		func() error {
			index, ent, err := state.KVSList(args.Key)
			if err != nil {
				return err
			}
			if acl != nil {
				ent = FilterDirEnt(acl, ent)
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

	acl, err := k.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	// Get the local state
	state := k.srv.fsm.State()
	return k.srv.blockingRPC(
		&args.QueryOptions,
		&reply.QueryMeta,
		state.GetKVSWatch(args.Prefix),
		func() error {
			index, keys, err := state.KVSListKeys(args.Prefix, args.Seperator)
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

			if acl != nil {
				keys = FilterKeys(acl, keys)
			}
			reply.Keys = keys
			return nil
		})
}
