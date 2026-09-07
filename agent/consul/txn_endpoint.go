// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-metrics/prometheus"

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
			requiresPreApply, err := nodeVerbValidate(op.Node.Verb)
			if err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}
			if !requiresPreApply {
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
			if err := t.vetNodeTxnOp(op.Node, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Service != nil:
			requiresPreApply, err := serviceVerbValidate(op.Service.Verb)
			if err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}
			if !requiresPreApply {
				break
			}

			service := &op.Service.Service
			if err := servicePreApplyValidate(service); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}

			if err := t.vetServiceTxnOp(op.Service, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Check != nil:
			requiresPreApply, err := checkVerbValidate(op.Check.Verb)
			if err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}
			if !requiresPreApply {
				break
			}

			checkPreApply(&op.Check.Check)

			// Check that the token has permissions for the given operation.
			if err := t.vetCheckTxnOp(op.Check, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Intention != nil:
			if err := intentionVerbValidate(op.Intention.Op); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		case op.Session != nil:
			if err := sessionVerbValidate(op.Session.Verb); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
				break
			}

			// Check that the token has permissions for the given operation.
			// Without this check a caller could destroy arbitrary sessions via
			// Txn.Apply without holding session:write, bypassing the ACL
			// enforcement performed by the Session.Apply endpoint.
			if err := t.vetSessionTxnOp(op.Session, authorizer); err != nil {
				errors = append(errors, &structs.TxnError{
					OpIndex: i,
					What:    err.Error(),
				})
			}
		default:
			errors = append(errors, &structs.TxnError{
				OpIndex: i,
				What:    "unknown operation type",
			})
		}
	}

	return errors
}

// vetNodeTxnOp applies the given ACL policy to a node transaction operation.
//
// If a request includes a Node.ID that resolves to an existing node, ACLs must be
// checked against that stored node name because NodeSet/NodeCAS ultimately mutate
// the object selected by the ID. If the request is renaming an existing node by
// ID, require write permission on both the stored name and requested name.
func (t *Txn) vetNodeTxnOp(op *structs.TxnNodeOp, authz resolver.Result) error {
	var requestCtx acl.AuthorizerContext
	op.FillAuthzContext(&requestCtx)

	if op.Node.ID != "" {
		_, existing, err := t.srv.fsm.State().GetNodeID(op.Node.ID, op.Node.GetEnterpriseMeta(), op.Node.PeerName)
		if err != nil {
			return fmt.Errorf("node lookup by ID failed: %w", err)
		}

		if existing != nil {
			var existingCtx acl.AuthorizerContext
			existing.FillAuthzContext(&existingCtx)

			if err := authz.ToAllowAuthorizer().NodeWriteAllowed(existing.Node, &existingCtx); err != nil {
				return err
			}

			// Preserve legitimate rename-by-ID behavior, but require access to
			// both the existing node and the requested replacement name.
			if !strings.EqualFold(existing.Node, op.Node.Node) {
				if err := authz.ToAllowAuthorizer().NodeWriteAllowed(op.Node.Node, &requestCtx); err != nil {
					return err
				}
			}

			return nil
		}
	}

	if err := authz.ToAllowAuthorizer().NodeWriteAllowed(op.Node.Node, &requestCtx); err != nil {
		return err
	}
	return nil
}

// vetSessionTxnOp applies ACL policy to a session transaction operation.
//
// Session txn mutations are applied by session ID, so (mirroring the
// Session.Apply SessionDestroy path) we authorize against the node the existing
// session is bound to. If no matching session exists the operation is a no-op
// and is allowed. Only the delete verb is currently supported.
func (t *Txn) vetSessionTxnOp(op *structs.TxnSessionOp, authz resolver.Result) error {
	var authzContext acl.AuthorizerContext
	op.Session.FillAuthzContext(&authzContext)

	_, existing, err := t.srv.fsm.State().SessionGet(nil, op.Session.ID, &op.Session.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("session lookup failed: %v", err)
	}
	if existing == nil {
		// Nothing to delete; treat as a no-op like Session.Apply does.
		return nil
	}

	return authz.ToAllowAuthorizer().SessionWriteAllowed(existing.Node, &authzContext)
}

// vetServiceTxnTarget applies target-aware ACL checks for service txn writes.
//
// servicePreApply validates the submitted service object and checks write access
// to the submitted service name. This helper additionally checks the stored
// service selected by Node + Service.ID + EnterpriseMeta + PeerName, because the
// state store mutates by service ID.
func (t *Txn) vetServiceTxnTarget(op *structs.TxnServiceOp, authz resolver.Result) error {
	_, existing, err := t.srv.fsm.State().NodeService(
		nil,
		op.Node,
		op.Service.ID,
		&op.Service.EnterpriseMeta,
		op.Service.PeerName,
	)
	if err != nil {
		return fmt.Errorf("service lookup failed: %w", err)
	}
	if existing == nil {
		// True create or no-op delete of a non-existent service. servicePreApply
		// already authorized the submitted canonical service name.
		return nil
	}

	var existingCtx acl.AuthorizerContext
	existing.FillAuthzContext(&existingCtx)

	if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(existing.Service, &existingCtx); err != nil {
		return err
	}

	if !strings.EqualFold(existing.Service, op.Service.Service) {
		return fmt.Errorf(
			"service ID %q on node %q is already registered as service %q; request service name %q does not match",
			op.Service.ID,
			op.Node,
			existing.Service,
			op.Service.Service,
		)
	}

	return nil
}

// vetServiceTxnOp applies ACL policy to a service transaction operation.
//
// Service txn mutations are applied by Service.ID, so authorization must not be
// based solely on the request's Service.Service field. If an existing service is
// found for the requested node/service ID, authorize against the existing service
// name and reject attempts to provide a different service name for the same ID.
func (t *Txn) vetServiceTxnOp(op *structs.TxnServiceOp, authz resolver.Result) error {
	service := &op.Service

	_, existing, err := t.srv.fsm.State().NodeService(
		nil,
		op.Node,
		service.ID,
		&service.EnterpriseMeta,
		service.PeerName,
	)
	if err != nil {
		return fmt.Errorf("service lookup failed: %v", err)
	}

	if existing != nil {
		if service.Service != existing.Service {
			return fmt.Errorf(
				"service name does not match existing service name for service ID %q on node %q",
				service.ID,
				op.Node,
			)
		}

		return serviceACLCheckNoConsulExemption(service, authz, func(ctx *acl.AuthorizerContext) {
			op.FillAuthzContext(ctx)
			existing.FillAuthzContext(ctx)
		})
	}

	// True create path: no existing service ID was found, so authorize against
	// the requested service name as before.
	return serviceACLCheckNoConsulExemption(service, authz, func(ctx *acl.AuthorizerContext) {
		op.FillAuthzContext(ctx)
		service.FillAuthzContext(ctx)
	})
}

// vetCheckTxnOp applies ACL policy to a check transaction operation.
//
// Check txn mutations are applied by CheckID. If a check already exists, ACLs
// must be evaluated against the existing check's real binding from the state
// store instead of trusting ServiceID or ServiceName supplied in the request.
func (t *Txn) vetCheckTxnOp(op *structs.TxnCheckOp, authz resolver.Result) error {
	check := &op.Check

	_, existing, err := t.srv.fsm.State().NodeCheck(
		check.Node,
		check.CheckID,
		&check.EnterpriseMeta,
		check.PeerName,
	)
	if err != nil {
		return fmt.Errorf("check lookup failed: %v", err)
	}

	if existing != nil {
		return t.vetCheckWrite(existing, op, authz)
	}

	return t.vetCheckWrite(check, op, authz)
}

func (t *Txn) vetCheckWrite(check *structs.HealthCheck, op *structs.TxnCheckOp, authz resolver.Result) error {
	var authzContext acl.AuthorizerContext

	if check.ServiceID == "" {
		// Node-level check.
		op.FillAuthzContext(&authzContext)
		check.FillAuthzContext(&authzContext)

		if err := authz.ToAllowAuthorizer().NodeWriteAllowed(check.Node, &authzContext); err != nil {
			return err
		}

		return nil
	}

	// Service-level check. Prefer resolving the actual service by ServiceID
	// from the state store so we don't trust Check.ServiceName from the
	// request. However, a check may be written in the same atomic transaction
	// that creates its service (the service won't yet be visible in the
	// committed FSM state at preCheck time); in that case fall back to
	// authorizing against the ServiceName declared on the incoming check.
	_, service, err := t.srv.fsm.State().NodeService(
		nil,
		check.Node,
		check.ServiceID,
		&check.EnterpriseMeta,
		check.PeerName,
	)
	if err != nil {
		return fmt.Errorf("service lookup failed: %v", err)
	}

	op.FillAuthzContext(&authzContext)

	var serviceName string
	if service != nil {
		// Authorize against the real, existing service binding.
		service.FillAuthzContext(&authzContext)
		serviceName = service.Service
	} else {
		// Service may be created earlier in the same transaction; fall back
		// to the ServiceName declared on the check op. Require it to be set
		// so callers can't bypass ACLs by omitting both the existing service
		// and the declared name.
		if check.ServiceName == "" {
			return fmt.Errorf("unknown service ID %q for check ID %q", check.ServiceID, check.CheckID)
		}
		check.FillAuthzContext(&authzContext)
		serviceName = check.ServiceName
	}

	if err := authz.ToAllowAuthorizer().ServiceWriteAllowed(serviceName, &authzContext); err != nil {
		return err
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
	reply.ResultsFilteredByACLs = total != len(reply.Results)

	// We have to do this ourselves since we are not doing a blocking RPC.
	t.srv.SetQueryMeta(&reply.QueryMeta, args.Token)

	return nil
}

// nodeVerbValidate checks for a known operation type. For certain operations,
// it also indicated if further "preApply" checks are required.
func nodeVerbValidate(op api.NodeOp) (bool, error) {
	// enumcover:api.NodeOp
	switch op {
	// Skip the pre-apply checks if this is a GET.
	case api.NodeGet:
		return false, nil
	case api.NodeSet, api.NodeCAS, api.NodeDelete, api.NodeDeleteCAS:
		return true, nil
	default:
		return false, fmt.Errorf("unknown node operation: %s", op)
	}
}

// serviceVerbValidate checks for a known operation type. For certain operations,
// it also indicated if further "preApply" checks are required.
func serviceVerbValidate(op api.ServiceOp) (bool, error) {
	// enumcover:api.ServiceOp
	switch op {
	// Skip the pre-apply checks if this is a GET.
	case api.ServiceGet:
		return false, nil
	case api.ServiceSet, api.ServiceCAS, api.ServiceDelete, api.ServiceDeleteCAS:
		return true, nil
	default:
		return false, fmt.Errorf("unknown service operation: %s", op)
	}
}

// checkVerbValidate checks for a known operation type. For certain operations,
// it also indicated if further "preApply" checks are required.
func checkVerbValidate(op api.CheckOp) (bool, error) {
	// enumcover:api.CheckOp
	switch op {
	// Skip the pre-apply checks if this is a GET.
	case api.CheckGet:
		return false, nil
	case api.CheckSet, api.CheckCAS, api.CheckDelete, api.CheckDeleteCAS:
		return true, nil
	default:
		return false, fmt.Errorf("unknown check operation: %s", op)
	}
}

// intentionVerbValidate checks for a known operation type.
func intentionVerbValidate(op structs.IntentionOp) error {
	// enumcover:structs.IntentionOp
	switch op {
	case structs.IntentionOpCreate, structs.IntentionOpDelete, structs.IntentionOpUpdate, structs.IntentionOpDeleteAll, structs.IntentionOpUpsert:
		return nil
	default:
		return fmt.Errorf("unknown intention operation: %s", op)
	}
}

// sessionVerbValidate checks for a known operation type.
func sessionVerbValidate(op api.SessionOp) error {
	// enumcover:api.SessionOp
	switch op {
	case api.SessionDelete:
		return nil
	default:
		return fmt.Errorf("unknown session operation: %s", op)
	}
}
