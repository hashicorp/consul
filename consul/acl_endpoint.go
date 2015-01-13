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

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return fmt.Errorf(aclDisabled)
	}

	// Verify token is permitted to modify ACLs
	if acl, err := a.srv.resolveToken(args.Token); err != nil {
		return err
	} else if acl == nil || !acl.ACLModify() {
		return permissionDeniedErr
	}

	switch args.Op {
	case structs.ACLSet:
		// Verify the ACL type
		switch args.ACL.Type {
		case structs.ACLTypeClient:
		case structs.ACLTypeManagement:
		default:
			return fmt.Errorf("Invalid ACL Type")
		}

		// Verify this is not a root ACL
		if acl.RootACL(args.ACL.ID) != nil {
			return fmt.Errorf("%s: Cannot modify root ACL", permissionDenied)
		}

		// Validate the rules compile
		_, err := acl.Parse(args.ACL.Rules)
		if err != nil {
			return fmt.Errorf("ACL rule compilation failed: %v", err)
		}

		// Check if this is an update
		state := a.srv.fsm.State()
		var existing *structs.ACL
		if args.ACL.ID != "" {
			_, existing, err = state.ACLGet(args.ACL.ID)
			if err != nil {
				a.srv.logger.Printf("[ERR] consul.acl: ACL lookup failed: %v", err)
				return err
			}
		}

		// If this is a create, generate a new ID. This must
		// be done prior to appending to the raft log, because the ID is not
		// deterministic. Once the entry is in the log, the state update MUST
		// be deterministic or the followers will not converge.
		if existing == nil {
			for {
				args.ACL.ID = generateUUID()
				_, acl, err := state.ACLGet(args.ACL.ID)
				if err != nil {
					a.srv.logger.Printf("[ERR] consul.acl: ACL lookup failed: %v", err)
					return err
				}
				if acl == nil {
					break
				}
			}
		}

	case structs.ACLDelete:
		if args.ACL.ID == "" {
			return fmt.Errorf("Missing ACL ID")
		} else if args.ACL.ID == anonymousToken {
			return fmt.Errorf("%s: Cannot delete anonymous token", permissionDenied)
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

	// Clear the cache if applicable
	if args.ACL.ID != "" {
		a.srv.aclAuthCache.ClearACL(args.ACL.ID)
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

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return fmt.Errorf(aclDisabled)
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
			} else {
				reply.ACLs = nil
			}
			return err
		})
}

// GetPolicy is used to retrieve a compiled policy object with a TTL. Does not
// support a blocking query.
func (a *ACL) GetPolicy(args *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
	if done, err := a.srv.forward("ACL.GetPolicy", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return fmt.Errorf(aclDisabled)
	}

	// Get the policy via the cache
	parent, policy, err := a.srv.aclAuthCache.GetACLPolicy(args.ACL)
	if err != nil {
		return err
	}

	// Generate an ETag
	conf := a.srv.config
	etag := fmt.Sprintf("%s:%s", parent, policy.ID)

	// Setup the response
	reply.ETag = etag
	reply.TTL = conf.ACLTTL
	a.srv.setQueryMeta(&reply.QueryMeta)

	// Only send the policy on an Etag mis-match
	if args.ETag != etag {
		reply.Parent = parent
		reply.Policy = policy
	}
	return nil
}

// List is used to list all the ACLs
func (a *ACL) List(args *structs.DCSpecificRequest,
	reply *structs.IndexedACLs) error {
	if done, err := a.srv.forward("ACL.List", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return fmt.Errorf(aclDisabled)
	}

	// Verify token is permitted to list ACLs
	if acl, err := a.srv.resolveToken(args.Token); err != nil {
		return err
	} else if acl == nil || !acl.ACLList() {
		return permissionDeniedErr
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
