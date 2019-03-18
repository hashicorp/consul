package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// Txn endpoint is used to perform multi-object atomic transactions.
type Txn struct {
	srv *Server
}

// preCheck is used to verify the incoming operations before any further
// processing takes place. This checks things like ACLs.
func (t *Txn) preCheck(authorizer acl.Authorizer, ops structs.TxnOps) structs.TxnErrors {
	var errors structs.TxnErrors

	// Perform the pre-apply checks for any KV operations.
	for i, op := range ops {
		switch {
		case op.KV != nil:
			ok, err := kvsPreApply(t.srv, authorizer, op.KV.Verb, &op.KV.DirEnt)
			if err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			} else if !ok {
				err = fmt.Errorf("failed to lock key %q due to lock delay", op.KV.DirEnt.Key)
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Node != nil:
			// Skip the pre-apply checks if this is a GET.
			if op.Node.Verb == api.NodeGet {
				break
			}

			node := op.Node.Node
			if err := nodePreApply(node.Node, string(node.ID)); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}

			// Check that the token has permissions for the given operation.
			if err := vetNodeTxnOp(op.Node, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Service != nil:
			// Skip the pre-apply checks if this is a GET.
			if op.Service.Verb == api.ServiceGet {
				break
			}

			service := &op.Service.Service
			if err := servicePreApply(service, nil); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}

			// Check that the token has permissions for the given operation.
			if err := vetServiceTxnOp(op.Service, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Check != nil:
			// Skip the pre-apply checks if this is a GET.
			if op.Check.Verb == api.CheckGet {
				break
			}

			checkPreApply(&op.Check.Check)

			// Check that the token has permissions for the given operation.
			if err := vetCheckTxnOp(op.Check, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		}
	}

	return errors
}

// Apply is used to apply multiple operations in a single, atomic transaction.
func (t *Txn) Apply(args *structs.TxnRequest, reply *structs.TxnResponse) error {
	if done, err := t.srv.forward("Txn.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"txn", "apply"}, time.Now())

	// Run the pre-checks before we send the transaction into Raft.
	authorizer, err := t.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	reply.Errors = t.preCheck(authorizer, args.Ops)
	if len(reply.Errors) > 0 {
		return nil
	}

	// Apply the update.
	resp, err := t.srv.raftApply(structs.TxnRequestType, args)
	if err != nil {
		t.srv.logger.Printf("[ERR] consul.txn: Apply failed: %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Convert the return type. This should be a cheap copy since we are
	// just taking the two slices.
	if txnResp, ok := resp.(structs.TxnResponse); ok {
		if authorizer != nil {
			txnResp.Results = FilterTxnResults(authorizer, txnResp.Results)
		}
		*reply = txnResp
	} else {
		return fmt.Errorf("unexpected return type %T", resp)
	}
	return nil
}

// Read is used to perform a read-only transaction that doesn't modify the state
// store. This is much more scalable since it doesn't go through Raft and
// supports staleness, so this should be preferred if you're just performing
// reads.
func (t *Txn) Read(args *structs.TxnReadRequest, reply *structs.TxnReadResponse) error {
	if done, err := t.srv.forward("Txn.Read", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"txn", "read"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	t.srv.setQueryMeta(&reply.QueryMeta)
	if args.RequireConsistent {
		if err := t.srv.consistentRead(); err != nil {
			return err
		}
	}

	// Run the pre-checks before we perform the read.
	authorizer, err := t.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	reply.Errors = t.preCheck(authorizer, args.Ops)
	if len(reply.Errors) > 0 {
		return nil
	}

	// Run the read transaction.
	state := t.srv.fsm.State()
	reply.Results, reply.Errors = state.TxnRO(args.Ops)
	if authorizer != nil {
		reply.Results = FilterTxnResults(authorizer, reply.Results)
	}
	return nil
}
