package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

var TxnSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"txn", "apply"},
		Help: "Measures the time spent applying a transaction operation.",
	},
	{
		Name: []string{"txn", "read"},
		Help: "Measures the time spent returning a read transaction.",
	},
}

// Txn endpoint is used to perform multi-object atomic transactions.
type Txn struct {
	srv    *Server
	logger hclog.Logger
}

// preCheck is used to verify the incoming operations before any further
// processing takes place. This checks things like ACLs.
func (t *Txn) preCheck(authorizer resolver.Result, ops structs.TxnOps) structs.TxnErrors {
	var errors structs.TxnErrors

	// Perform the pre-apply checks for any KV operations.
	for i, op := range ops {
		switch {
		case op.KV != nil:
			ok, err := kvsPreApply(t.logger, t.srv, authorizer, op.KV.Verb, &op.KV.DirEnt)
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
			if err := servicePreApply(service, authorizer, op.Service.FillAuthzContext); err != nil {
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

// vetNodeTxnOp applies the given ACL policy to a node transaction operation.
func vetNodeTxnOp(op *structs.TxnNodeOp, authz resolver.Result) error {
	var authzContext acl.AuthorizerContext
	op.FillAuthzContext(&authzContext)

	if err := authz.ToAllowAuthorizer().NodeWriteAllowed(op.Node.Node, &authzContext); err != nil {
		return err
	}
	return nil
}

// vetCheckTxnOp applies the given ACL policy to a check transaction operation.
func vetCheckTxnOp(op *structs.TxnCheckOp, authz resolver.Result) error {
	var authzContext acl.AuthorizerContext
	op.FillAuthzContext(&authzContext)

	if op.Check.ServiceID == "" {
		// Node-level check.
		if err := authz.ToAllowAuthorizer().NodeWriteAllowed(op.Check.Node, &authzContext); err != nil {
			return err
		}
	} else {
		// Service-level check.
		if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(op.Check.ServiceName, &authzContext); err != nil {
			return err
		}
	}
	return nil
}

// Apply is used to apply multiple operations in a single, atomic transaction.
func (t *Txn) Apply(args *structs.TxnRequest, reply *structs.TxnResponse) error {
	if done, err := t.srv.ForwardRPC("Txn.Apply", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"txn", "apply"}, time.Now())

	// Run the pre-checks before we send the transaction into Raft.
	authz, err := t.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	reply.Errors = t.preCheck(authz, args.Ops)
	if len(reply.Errors) > 0 {
		return nil
	}

	// Apply the update.
	resp, err := t.srv.raftApply(structs.TxnRequestType, args)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}

	// Convert the return type. This should be a cheap copy since we are
	// just taking the two slices.
	if txnResp, ok := resp.(structs.TxnResponse); ok {
		txnResp.Results = FilterTxnResults(authz, txnResp.Results)
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
	if done, err := t.srv.ForwardRPC("Txn.Read", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"txn", "read"}, time.Now())

	// We have to do this ourselves since we are not doing a blocking RPC.
	if args.RequireConsistent {
		if err := t.srv.ConsistentRead(); err != nil {
			return err
		}
	}

	// Run the pre-checks before we perform the read.
	authz, err := t.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	// There are currently two different ways we handle permission issues.
	//
	// For simple reads such as KVGet and KVGetTree, the txn succeeds but the
	// offending results are omitted. For more involved operations such as
	// KVCheckIndex, the txn fails and permission denied errors are returned.
	//
	// TODO: Maybe we should unify these, or at least cover it in the docs?
	reply.Errors = t.preCheck(authz, args.Ops)
	if len(reply.Errors) > 0 {
		return nil
	}

	// Run the read transaction.
	state := t.srv.fsm.State()
	reply.Results, reply.Errors = state.TxnRO(args.Ops)

	total := len(reply.Results)
	reply.Results = FilterTxnResults(authz, reply.Results)
	reply.QueryMeta.ResultsFilteredByACLs = total != len(reply.Results)

	// We have to do this ourselves since we are not doing a blocking RPC.
	t.srv.SetQueryMeta(&reply.QueryMeta, args.Token)

	return nil
}
