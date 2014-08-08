package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/consul/structs"
)

// ACL endpoint is used to manipulate ACLs
type ACL struct {
	srv *Server
}

// Apply is used to apply a modifying request to the data store. This should
// only be used for operations that modify the data
func (a *ACL) Apply(args *structs.ACLRequest, reply *string) error {
	if done, err := a.srv.forward("ACL.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "acl", "apply"}, time.Now())

	switch args.Op {
	case structs.ACLSet:
		// Verify the ACL type
		switch args.ACL.Type {
		case structs.ACLTypeClient:
		case structs.ACLTypeManagement:
		default:
			return fmt.Errorf("Invalid ACL Type")
		}

		// Validate the rules compile
		_, err := acl.Parse(args.ACL.Rules)
		if err != nil {
			return fmt.Errorf("ACL rule compilation failed: %v", err)
		}

	case structs.ACLDelete:
		if args.ACL.ID == "" {
			return fmt.Errorf("Missing ACL ID")
		}

	default:
		return fmt.Errorf("Invalid ACL Operation")
	}

	// Apply the update
	resp, err := a.srv.raftApply(structs.ACLRequestType, args)
	if err != nil {
		a.srv.logger.Printf("[ERR] consul.acl: Apply failed: %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Check if the return type is a string
	if respString, ok := resp.(string); ok {
		*reply = respString
	}
	return nil
}

// Get is used to retrieve a single ACL
func (a *ACL) Get(args *structs.ACLSpecificRequest,
	reply *structs.IndexedACLs) error {
	if done, err := a.srv.forward("ACL.Get", args, args, reply); done {
		return err
	}

	// Get the local state
	state := a.srv.fsm.State()
	return a.srv.blockingRPC(&args.QueryOptions,
		&reply.QueryMeta,
		state.QueryTables("ACLGet"),
		func() error {
			index, acl, err := state.ACLGet(args.ACL)
			reply.Index = index
			if acl != nil {
				reply.ACLs = structs.ACLs{acl}
			}
			return err
		})
}

// GetPolicy is used to retrieve a compiled policy object with a TTL. Does not
// support a blocking query.
func (a *ACL) GetPolicy(args *structs.ACLSpecificRequest, reply *structs.ACLPolicy) error {
	if done, err := a.srv.forward("ACL.GetPolicy", args, args, reply); done {
		return err
	}

	// Get the policy via the cache
	policy, err := a.srv.aclAuthCache.GetACLPolicy(args.ACL)
	if err != nil {
		return err
	}

	// Setup the response
	conf := a.srv.config
	reply.Policy = policy
	reply.Root = conf.ACLDefaultPolicy
	reply.TTL = conf.ACLTTL
	a.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// List is used to list all the ACLs
func (a *ACL) List(args *structs.DCSpecificRequest,
	reply *structs.IndexedACLs) error {
	if done, err := a.srv.forward("ACL.List", args, args, reply); done {
		return err
	}

	// Get the local state
	state := a.srv.fsm.State()
	return a.srv.blockingRPC(&args.QueryOptions,
		&reply.QueryMeta,
		state.QueryTables("ACLList"),
		func() error {
			var err error
			reply.Index, reply.ACLs, err = state.ACLList()
			return err
		})
}
