// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
)

type aclTokenReplicator struct {
	local   structs.ACLTokens
	remote  structs.ACLTokenListStubs
	updated []*structs.ACLToken
}

var _ aclTypeReplicator = (*aclTokenReplicator)(nil)

func (r *aclTokenReplicator) Type() structs.ACLReplicationType { return structs.ACLReplicateTokens }
func (r *aclTokenReplicator) SingularNoun() string             { return "token" }
func (r *aclTokenReplicator) PluralNoun() string               { return "tokens" }

func (r *aclTokenReplicator) FetchRemote(srv *Server, lastRemoteIndex uint64) (int, uint64, error) {
	r.remote = nil

	remote, err := srv.fetchACLTokens(lastRemoteIndex)
	if err != nil {
		return 0, 0, err
	}

	r.remote = remote.Tokens
	return len(remote.Tokens), remote.QueryMeta.Index, nil
}

func (r *aclTokenReplicator) FetchLocal(srv *Server) (int, uint64, error) {
	r.local = nil

	idx, local, err := srv.fsm.State().ACLTokenList(nil, false, true, "", "", "", nil, srv.replicationEnterpriseMeta())
	if err != nil {
		return 0, 0, err
	}

	// Do not filter by expiration times. Wait until the tokens are explicitly
	// deleted.

	r.local = local
	return len(local), idx, nil
}

func (r *aclTokenReplicator) SortState() (int, int) {
	r.local.Sort()
	r.remote.Sort()

	return len(r.local), len(r.remote)
}
func (r *aclTokenReplicator) LocalMeta(i int) (id string, modIndex uint64, hash []byte) {
	v := r.local[i]
	return v.AccessorID, v.ModifyIndex, v.Hash
}
func (r *aclTokenReplicator) RemoteMeta(i int) (id string, modIndex uint64, hash []byte) {
	v := r.remote[i]
	return v.AccessorID, v.ModifyIndex, v.Hash
}

func (r *aclTokenReplicator) FetchUpdated(srv *Server, updates []string) (int, error) {
	r.updated = nil

	if len(updates) > 0 {
		tokens, err := srv.fetchACLTokensBatch(updates)
		if err != nil {
			return 0, err
		} else if tokens.Redacted {
			return 0, errContainsRedactedData
		}

		// Do not filter by expiration times. Wait until the tokens are
		// explicitly deleted.

		r.updated = tokens.Tokens
	}

	return len(r.updated), nil
}

func (r *aclTokenReplicator) DeleteLocalBatch(srv *Server, batch []string) error {
	req := structs.ACLTokenBatchDeleteRequest{
		TokenIDs: batch,
	}

	_, err := srv.leaderRaftApply("ACL.TokenDelete", structs.ACLTokenDeleteRequestType, &req)
	return err
}

func (r *aclTokenReplicator) LenPendingUpdates() int {
	return len(r.updated)
}

func (r *aclTokenReplicator) PendingUpdateEstimatedSize(i int) int {
	return r.updated[i].EstimateSize()
}

func (r *aclTokenReplicator) PendingUpdateIsRedacted(i int) bool {
	return r.updated[i].SecretID == aclfilter.RedactedToken
}

func (r *aclTokenReplicator) UpdateLocalBatch(ctx context.Context, srv *Server, start, end int) error {
	req := structs.ACLTokenBatchSetRequest{
		Tokens:            r.updated[start:end],
		CAS:               false,
		AllowMissingLinks: true,
		FromReplication:   true,
	}

	_, err := srv.leaderRaftApply("ACL.TokenSet", structs.ACLTokenSetRequestType, &req)
	return err
}

///////////////////////

type aclPolicyReplicator struct {
	local   structs.ACLPolicies
	remote  structs.ACLPolicyListStubs
	updated []*structs.ACLPolicy
}

var _ aclTypeReplicator = (*aclPolicyReplicator)(nil)

func (r *aclPolicyReplicator) Type() structs.ACLReplicationType { return structs.ACLReplicatePolicies }
func (r *aclPolicyReplicator) SingularNoun() string             { return "policy" }
func (r *aclPolicyReplicator) PluralNoun() string               { return "policies" }

func (r *aclPolicyReplicator) FetchRemote(srv *Server, lastRemoteIndex uint64) (int, uint64, error) {
	r.remote = nil

	remote, err := srv.fetchACLPolicies(lastRemoteIndex)
	if err != nil {
		return 0, 0, err
	}

	r.remote = remote.Policies
	return len(remote.Policies), remote.QueryMeta.Index, nil
}

func (r *aclPolicyReplicator) FetchLocal(srv *Server) (int, uint64, error) {
	r.local = nil

	idx, local, err := srv.fsm.State().ACLPolicyList(nil, srv.replicationEnterpriseMeta())
	if err != nil {
		return 0, 0, err
	}

	r.local = local
	return len(local), idx, nil
}

func (r *aclPolicyReplicator) SortState() (int, int) {
	r.local.Sort()
	r.remote.Sort()

	return len(r.local), len(r.remote)
}
func (r *aclPolicyReplicator) LocalMeta(i int) (id string, modIndex uint64, hash []byte) {
	v := r.local[i]
	return v.ID, v.ModifyIndex, v.Hash
}
func (r *aclPolicyReplicator) RemoteMeta(i int) (id string, modIndex uint64, hash []byte) {
	v := r.remote[i]
	return v.ID, v.ModifyIndex, v.Hash
}

func (r *aclPolicyReplicator) FetchUpdated(srv *Server, updates []string) (int, error) {
	r.updated = nil

	if len(updates) > 0 {
		policies, err := srv.fetchACLPoliciesBatch(updates)
		if err != nil {
			return 0, err
		}
		r.updated = policies.Policies
	}

	return len(r.updated), nil
}

func (r *aclPolicyReplicator) DeleteLocalBatch(srv *Server, batch []string) error {
	req := structs.ACLPolicyBatchDeleteRequest{
		PolicyIDs: batch,
	}

	_, err := srv.leaderRaftApply("ACL.PolicyDelete", structs.ACLPolicyDeleteRequestType, &req)
	return err
}

func (r *aclPolicyReplicator) LenPendingUpdates() int {
	return len(r.updated)
}

func (r *aclPolicyReplicator) PendingUpdateEstimatedSize(i int) int {
	return r.updated[i].EstimateSize()
}

func (r *aclPolicyReplicator) PendingUpdateIsRedacted(i int) bool {
	return false
}

func (r *aclPolicyReplicator) UpdateLocalBatch(ctx context.Context, srv *Server, start, end int) error {
	req := structs.ACLPolicyBatchSetRequest{
		Policies: r.updated[start:end],
	}

	_, err := srv.leaderRaftApply("ACL.PolicySet", structs.ACLPolicySetRequestType, &req)
	return err
}

////////////////////////////////

type aclRoleReplicator struct {
	local   structs.ACLRoles
	remote  structs.ACLRoles
	updated []*structs.ACLRole
}

var _ aclTypeReplicator = (*aclRoleReplicator)(nil)

func (r *aclRoleReplicator) Type() structs.ACLReplicationType { return structs.ACLReplicateRoles }
func (r *aclRoleReplicator) SingularNoun() string             { return "role" }
func (r *aclRoleReplicator) PluralNoun() string               { return "roles" }

func (r *aclRoleReplicator) FetchRemote(srv *Server, lastRemoteIndex uint64) (int, uint64, error) {
	r.remote = nil

	remote, err := srv.fetchACLRoles(lastRemoteIndex)
	if err != nil {
		return 0, 0, err
	}

	r.remote = remote.Roles
	return len(remote.Roles), remote.QueryMeta.Index, nil
}

func (r *aclRoleReplicator) FetchLocal(srv *Server) (int, uint64, error) {
	r.local = nil

	idx, local, err := srv.fsm.State().ACLRoleList(nil, "", srv.replicationEnterpriseMeta())
	if err != nil {
		return 0, 0, err
	}

	r.local = local
	return len(local), idx, nil
}

func (r *aclRoleReplicator) SortState() (int, int) {
	r.local.Sort()
	r.remote.Sort()

	return len(r.local), len(r.remote)
}
func (r *aclRoleReplicator) LocalMeta(i int) (id string, modIndex uint64, hash []byte) {
	v := r.local[i]
	return v.ID, v.ModifyIndex, v.Hash
}
func (r *aclRoleReplicator) RemoteMeta(i int) (id string, modIndex uint64, hash []byte) {
	v := r.remote[i]
	return v.ID, v.ModifyIndex, v.Hash
}

func (r *aclRoleReplicator) FetchUpdated(srv *Server, updates []string) (int, error) {
	r.updated = nil

	if len(updates) > 0 {
		// Since ACLRoles do not have a "list entry" variation, all of the data
		// to replicate a role is already present in the "r.remote" list.
		//
		// We avoid a second query by just repurposing the data we already have
		// access to in a way that is compatible with the generic ACL type
		// replicator.
		keep := make(map[string]struct{})
		for _, id := range updates {
			keep[id] = struct{}{}
		}

		subset := make([]*structs.ACLRole, 0, len(updates))
		for _, role := range r.remote {
			if _, ok := keep[role.ID]; ok {
				subset = append(subset, role)
			}
		}

		if len(subset) != len(keep) { // only possible via programming bug
			for _, role := range subset {
				delete(keep, role.ID)
			}
			missing := make([]string, 0, len(keep))
			for id := range keep {
				missing = append(missing, id)
			}
			return 0, fmt.Errorf("role replication trying to replicated uncached roles with IDs: %v", missing)
		}
		r.updated = subset
	}

	return len(r.updated), nil
}

func (r *aclRoleReplicator) DeleteLocalBatch(srv *Server, batch []string) error {
	req := structs.ACLRoleBatchDeleteRequest{
		RoleIDs: batch,
	}

	_, err := srv.leaderRaftApply("ACL.RoleDelete", structs.ACLRoleDeleteRequestType, &req)
	return err
}

func (r *aclRoleReplicator) LenPendingUpdates() int {
	return len(r.updated)
}

func (r *aclRoleReplicator) PendingUpdateEstimatedSize(i int) int {
	return r.updated[i].EstimateSize()
}

func (r *aclRoleReplicator) PendingUpdateIsRedacted(i int) bool {
	return false
}

func (r *aclRoleReplicator) UpdateLocalBatch(ctx context.Context, srv *Server, start, end int) error {
	req := structs.ACLRoleBatchSetRequest{
		Roles:             r.updated[start:end],
		AllowMissingLinks: true,
	}

	_, err := srv.leaderRaftApply("ACL.RoleSet", structs.ACLRoleSetRequestType, &req)

	return err
}
