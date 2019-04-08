package consul

import (
	"bytes"
	"context"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// aclReplicationMaxRetryBackoff is the max number of seconds to sleep between ACL replication RPC errors
	aclReplicationMaxRetryBackoff = 64
)

func diffACLPolicies(local structs.ACLPolicies, remote structs.ACLPolicyListStubs, lastRemoteIndex uint64) ([]string, []string) {
	local.Sort()
	remote.Sort()

	var deletions []string
	var updates []string
	var localIdx int
	var remoteIdx int
	for localIdx, remoteIdx = 0, 0; localIdx < len(local) && remoteIdx < len(remote); {
		if local[localIdx].ID == remote[remoteIdx].ID {
			// policy is in both the local and remote state - need to check raft indices and the Hash
			if remote[remoteIdx].ModifyIndex > lastRemoteIndex && !bytes.Equal(remote[remoteIdx].Hash, local[localIdx].Hash) {
				updates = append(updates, remote[remoteIdx].ID)
			}
			// increment both indices when equal
			localIdx += 1
			remoteIdx += 1
		} else if local[localIdx].ID < remote[remoteIdx].ID {
			// policy no longer in remoted state - needs deleting
			deletions = append(deletions, local[localIdx].ID)

			// increment just the local index
			localIdx += 1
		} else {
			// local state doesn't have this policy - needs updating
			updates = append(updates, remote[remoteIdx].ID)

			// increment just the remote index
			remoteIdx += 1
		}
	}

	for ; localIdx < len(local); localIdx += 1 {
		deletions = append(deletions, local[localIdx].ID)
	}

	for ; remoteIdx < len(remote); remoteIdx += 1 {
		updates = append(updates, remote[remoteIdx].ID)
	}

	return deletions, updates
}

func (s *Server) deleteLocalACLPolicies(deletions []string, ctx context.Context) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	for i := 0; i < len(deletions); i += aclBatchDeleteSize {
		req := structs.ACLPolicyBatchDeleteRequest{}

		if i+aclBatchDeleteSize > len(deletions) {
			req.PolicyIDs = deletions[i:]
		} else {
			req.PolicyIDs = deletions[i : i+aclBatchDeleteSize]
		}

		resp, err := s.raftApply(structs.ACLPolicyDeleteRequestType, &req)
		if err != nil {
			return false, fmt.Errorf("Failed to apply policy deletions: %v", err)
		}
		if respErr, ok := resp.(error); ok && err != nil {
			return false, fmt.Errorf("Failed to apply policy deletions: %v", respErr)
		}

		if i+aclBatchDeleteSize < len(deletions) {
			select {
			case <-ctx.Done():
				return true, nil
			case <-ticker.C:
				// do nothing - ready for the next batch
			}
		}
	}

	return false, nil
}

func (s *Server) updateLocalACLPolicies(policies structs.ACLPolicies, ctx context.Context) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	// outer loop handles submitting a batch
	for batchStart := 0; batchStart < len(policies); {
		// inner loop finds the last element to include in this batch.
		batchSize := 0
		batchEnd := batchStart
		for ; batchEnd < len(policies) && batchSize < aclBatchUpsertSize; batchEnd += 1 {
			batchSize += policies[batchEnd].EstimateSize()
		}

		req := structs.ACLPolicyBatchSetRequest{
			Policies: policies[batchStart:batchEnd],
		}

		resp, err := s.raftApply(structs.ACLPolicySetRequestType, &req)
		if err != nil {
			return false, fmt.Errorf("Failed to apply policy upserts: %v", err)
		}
		if respErr, ok := resp.(error); ok && respErr != nil {
			return false, fmt.Errorf("Failed to apply policy upsert: %v", respErr)
		}
		s.logger.Printf("[DEBUG] acl: policy replication - upserted 1 batch with %d policies of size %d", batchEnd-batchStart, batchSize)

		// policies[batchEnd] wasn't include as the slicing doesn't include the element at the stop index
		batchStart = batchEnd

		// prevent waiting if we are done
		if batchEnd < len(policies) {
			select {
			case <-ctx.Done():
				return true, nil
			case <-ticker.C:
				// nothing to do - just rate limiting
			}
		}
	}
	return false, nil
}

func (s *Server) fetchACLPoliciesBatch(policyIDs []string) (*structs.ACLPolicyBatchResponse, error) {
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter: s.config.ACLDatacenter,
		PolicyIDs:  policyIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ReplicationToken(),
		},
	}

	var response structs.ACLPolicyBatchResponse
	if err := s.RPC("ACL.PolicyBatchRead", &req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Server) fetchACLPolicies(lastRemoteIndex uint64) (*structs.ACLPolicyListResponse, error) {
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "policy", "fetch"}, time.Now())

	req := structs.ACLPolicyListRequest{
		Datacenter: s.config.ACLDatacenter,
		QueryOptions: structs.QueryOptions{
			AllowStale:    true,
			MinQueryIndex: lastRemoteIndex,
			Token:         s.tokens.ReplicationToken(),
		},
	}

	var response structs.ACLPolicyListResponse
	if err := s.RPC("ACL.PolicyList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

type tokenDiffResults struct {
	LocalDeletes  []string
	LocalUpserts  []string
	LocalSkipped  int
	RemoteSkipped int
}

func diffACLTokens(local structs.ACLTokens, remote structs.ACLTokenListStubs, lastRemoteIndex uint64) tokenDiffResults {
	// Note: items with empty AccessorIDs will bubble up to the top.
	local.Sort()
	remote.Sort()

	var res tokenDiffResults
	var localIdx int
	var remoteIdx int
	for localIdx, remoteIdx = 0, 0; localIdx < len(local) && remoteIdx < len(remote); {
		if local[localIdx].AccessorID == "" {
			res.LocalSkipped++
			localIdx += 1
			continue
		}
		if remote[remoteIdx].AccessorID == "" {
			res.RemoteSkipped++
			remoteIdx += 1
			continue
		}
		if local[localIdx].AccessorID == remote[remoteIdx].AccessorID {
			// policy is in both the local and remote state - need to check raft indices and Hash
			if remote[remoteIdx].ModifyIndex > lastRemoteIndex && !bytes.Equal(remote[remoteIdx].Hash, local[localIdx].Hash) {
				res.LocalUpserts = append(res.LocalUpserts, remote[remoteIdx].AccessorID)
			}
			// increment both indices when equal
			localIdx += 1
			remoteIdx += 1
		} else if local[localIdx].AccessorID < remote[remoteIdx].AccessorID {
			// policy no longer in remoted state - needs deleting
			res.LocalDeletes = append(res.LocalDeletes, local[localIdx].AccessorID)

			// increment just the local index
			localIdx += 1
		} else {
			// local state doesn't have this policy - needs updating
			res.LocalUpserts = append(res.LocalUpserts, remote[remoteIdx].AccessorID)

			// increment just the remote index
			remoteIdx += 1
		}
	}

	for ; localIdx < len(local); localIdx += 1 {
		if local[localIdx].AccessorID != "" {
			res.LocalDeletes = append(res.LocalDeletes, local[localIdx].AccessorID)
		} else {
			res.LocalSkipped++
		}
	}

	for ; remoteIdx < len(remote); remoteIdx += 1 {
		if remote[remoteIdx].AccessorID != "" {
			res.LocalUpserts = append(res.LocalUpserts, remote[remoteIdx].AccessorID)
		} else {
			res.RemoteSkipped++
		}
	}

	return res
}

func (s *Server) deleteLocalACLTokens(deletions []string, ctx context.Context) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	for i := 0; i < len(deletions); i += aclBatchDeleteSize {
		req := structs.ACLTokenBatchDeleteRequest{}

		if i+aclBatchDeleteSize > len(deletions) {
			req.TokenIDs = deletions[i:]
		} else {
			req.TokenIDs = deletions[i : i+aclBatchDeleteSize]
		}

		resp, err := s.raftApply(structs.ACLTokenDeleteRequestType, &req)
		if err != nil {
			return false, fmt.Errorf("Failed to apply token deletions: %v", err)
		}
		if respErr, ok := resp.(error); ok && err != nil {
			return false, fmt.Errorf("Failed to apply token deletions: %v", respErr)
		}

		if i+aclBatchDeleteSize < len(deletions) {
			select {
			case <-ctx.Done():
				return true, nil
			case <-ticker.C:
				// do nothing - ready for the next batch
			}
		}
	}

	return false, nil
}

func (s *Server) updateLocalACLTokens(tokens structs.ACLTokens, ctx context.Context) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	// outer loop handles submitting a batch
	for batchStart := 0; batchStart < len(tokens); {
		// inner loop finds the last element to include in this batch.
		batchSize := 0
		batchEnd := batchStart
		for ; batchEnd < len(tokens) && batchSize < aclBatchUpsertSize; batchEnd += 1 {
			if tokens[batchEnd].SecretID == redactedToken {
				return false, fmt.Errorf("Detected redacted token secrets: stopping token update round - verify that the replication token in use has acl:write permissions.")
			}
			batchSize += tokens[batchEnd].EstimateSize()
		}

		req := structs.ACLTokenBatchSetRequest{
			Tokens: tokens[batchStart:batchEnd],
			CAS:    false,
		}

		resp, err := s.raftApply(structs.ACLTokenSetRequestType, &req)
		if err != nil {
			return false, fmt.Errorf("Failed to apply token upserts: %v", err)
		}
		if respErr, ok := resp.(error); ok && respErr != nil {
			return false, fmt.Errorf("Failed to apply token upserts: %v", respErr)
		}

		s.logger.Printf("[DEBUG] acl: token replication - upserted 1 batch with %d tokens of size %d", batchEnd-batchStart, batchSize)

		// tokens[batchEnd] wasn't include as the slicing doesn't include the element at the stop index
		batchStart = batchEnd

		// prevent waiting if we are done
		if batchEnd < len(tokens) {
			select {
			case <-ctx.Done():
				return true, nil
			case <-ticker.C:
				// nothing to do - just rate limiting here
			}
		}
	}
	return false, nil
}

func (s *Server) fetchACLTokensBatch(tokenIDs []string) (*structs.ACLTokenBatchResponse, error) {
	req := structs.ACLTokenBatchGetRequest{
		Datacenter:  s.config.ACLDatacenter,
		AccessorIDs: tokenIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ReplicationToken(),
		},
	}

	var response structs.ACLTokenBatchResponse
	if err := s.RPC("ACL.TokenBatchRead", &req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Server) fetchACLTokens(lastRemoteIndex uint64) (*structs.ACLTokenListResponse, error) {
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "token", "fetch"}, time.Now())

	req := structs.ACLTokenListRequest{
		Datacenter: s.config.ACLDatacenter,
		QueryOptions: structs.QueryOptions{
			AllowStale:    true,
			MinQueryIndex: lastRemoteIndex,
			Token:         s.tokens.ReplicationToken(),
		},
		IncludeLocal:  false,
		IncludeGlobal: true,
	}

	var response structs.ACLTokenListResponse
	if err := s.RPC("ACL.TokenList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *Server) replicateACLPolicies(lastRemoteIndex uint64, ctx context.Context) (uint64, bool, error) {
	remote, err := s.fetchACLPolicies(lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve remote ACL policies: %v", err)
	}

	s.logger.Printf("[DEBUG] acl: finished fetching policies tokens: %d", len(remote.Policies))

	// Need to check if we should be stopping. This will be common as the fetching process is a blocking
	// RPC which could have been hanging around for a long time and during that time leadership could
	// have been lost.
	select {
	case <-ctx.Done():
		return 0, true, nil
	default:
		// do nothing
	}

	// Measure everything after the remote query, which can block for long
	// periods of time. This metric is a good measure of how expensive the
	// replication process is.
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "policy", "apply"}, time.Now())

	_, local, err := s.fsm.State().ACLPolicyList(nil)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve local ACL policies: %v", err)
	}

	// If the remote index ever goes backwards, it's a good indication that
	// the remote side was rebuilt and we should do a full sync since we
	// can't make any assumptions about what's going on.
	if remote.QueryMeta.Index < lastRemoteIndex {
		s.logger.Printf("[WARN] consul: ACL policy replication remote index moved backwards (%d to %d), forcing a full ACL policy sync", lastRemoteIndex, remote.QueryMeta.Index)
		lastRemoteIndex = 0
	}

	s.logger.Printf("[DEBUG] acl: policy replication - local: %d, remote: %d", len(local), len(remote.Policies))
	// Calculate the changes required to bring the state into sync and then
	// apply them.
	deletions, updates := diffACLPolicies(local, remote.Policies, lastRemoteIndex)

	s.logger.Printf("[DEBUG] acl: policy replication - deletions: %d, updates: %d", len(deletions), len(updates))

	var policies *structs.ACLPolicyBatchResponse
	if len(updates) > 0 {
		policies, err = s.fetchACLPoliciesBatch(updates)
		if err != nil {
			return 0, false, fmt.Errorf("failed to retrieve ACL policy updates: %v", err)
		}
		s.logger.Printf("[DEBUG] acl: policy replication - downloaded %d policies", len(policies.Policies))
	}

	if len(deletions) > 0 {
		s.logger.Printf("[DEBUG] acl: policy replication - performing deletions")

		exit, err := s.deleteLocalACLPolicies(deletions, ctx)
		if exit {
			return 0, true, nil
		}
		if err != nil {
			return 0, false, fmt.Errorf("failed to delete local ACL policies: %v", err)
		}
		s.logger.Printf("[DEBUG] acl: policy replication - finished deletions")
	}

	if len(updates) > 0 {
		s.logger.Printf("[DEBUG] acl: policy replication - performing updates")
		exit, err := s.updateLocalACLPolicies(policies.Policies, ctx)
		if exit {
			return 0, true, nil
		}
		if err != nil {
			return 0, false, fmt.Errorf("failed to update local ACL policies: %v", err)
		}
		s.logger.Printf("[DEBUG] acl: policy replication - finished updates")
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remote.QueryMeta.Index, false, nil
}

func (s *Server) replicateACLTokens(lastRemoteIndex uint64, ctx context.Context) (uint64, bool, error) {
	remote, err := s.fetchACLTokens(lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve remote ACL tokens: %v", err)
	}

	s.logger.Printf("[DEBUG] acl: finished fetching remote tokens: %d", len(remote.Tokens))

	// Need to check if we should be stopping. This will be common as the fetching process is a blocking
	// RPC which could have been hanging around for a long time and during that time leadership could
	// have been lost.
	select {
	case <-ctx.Done():
		return 0, true, nil
	default:
		// do nothing
	}

	// Measure everything after the remote query, which can block for long
	// periods of time. This metric is a good measure of how expensive the
	// replication process is.
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "token", "apply"}, time.Now())

	_, local, err := s.fsm.State().ACLTokenList(nil, false, true, "")
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve local ACL tokens: %v", err)
	}
	// Do not filter by expiration times. Wait until the tokens are explicitly deleted.

	// If the remote index ever goes backwards, it's a good indication that
	// the remote side was rebuilt and we should do a full sync since we
	// can't make any assumptions about what's going on.
	if remote.QueryMeta.Index < lastRemoteIndex {
		s.logger.Printf("[WARN] consul: ACL token replication remote index moved backwards (%d to %d), forcing a full ACL token sync", lastRemoteIndex, remote.QueryMeta.Index)
		lastRemoteIndex = 0
	}

	s.logger.Printf("[DEBUG] acl: token replication - local: %d, remote: %d", len(local), len(remote.Tokens))

	// Calculate the changes required to bring the state into sync and then
	// apply them.
	res := diffACLTokens(local, remote.Tokens, lastRemoteIndex)
	if res.LocalSkipped > 0 || res.RemoteSkipped > 0 {
		s.logger.Printf("[DEBUG] acl: token replication - deletions: %d, updates: %d, skipped: %d, skippedRemote: %d",
			len(res.LocalDeletes), len(res.LocalUpserts), res.LocalSkipped, res.RemoteSkipped)
	} else {
		s.logger.Printf("[DEBUG] acl: token replication - deletions: %d, updates: %d", len(res.LocalDeletes), len(res.LocalUpserts))
	}

	var tokens *structs.ACLTokenBatchResponse
	if len(res.LocalUpserts) > 0 {
		tokens, err = s.fetchACLTokensBatch(res.LocalUpserts)
		if err != nil {
			return 0, false, fmt.Errorf("failed to retrieve ACL token updates: %v", err)
		} else if tokens.Redacted {
			return 0, false, fmt.Errorf("failed to retrieve unredacted tokens - replication token in use does not grant acl:write")
		}

		s.logger.Printf("[DEBUG] acl: token replication - downloaded %d tokens", len(tokens.Tokens))
	}

	if len(res.LocalDeletes) > 0 {
		s.logger.Printf("[DEBUG] acl: token replication - performing deletions")

		exit, err := s.deleteLocalACLTokens(res.LocalDeletes, ctx)
		if exit {
			return 0, true, nil
		}
		if err != nil {
			return 0, false, fmt.Errorf("failed to delete local ACL tokens: %v", err)
		}
		s.logger.Printf("[DEBUG] acl: token replication - finished deletions")
	}

	if len(res.LocalUpserts) > 0 {
		s.logger.Printf("[DEBUG] acl: token replication - performing updates")
		exit, err := s.updateLocalACLTokens(tokens.Tokens, ctx)
		if exit {
			return 0, true, nil
		}
		if err != nil {
			return 0, false, fmt.Errorf("failed to update local ACL tokens: %v", err)
		}
		s.logger.Printf("[DEBUG] acl: token replication - finished updates")
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remote.QueryMeta.Index, false, nil
}

// IsACLReplicationEnabled returns true if ACL replication is enabled.
// DEPRECATED (ACL-Legacy-Compat) - with new ACLs at least policy replication is required
func (s *Server) IsACLReplicationEnabled() bool {
	authDC := s.config.ACLDatacenter
	return len(authDC) > 0 && (authDC != s.config.Datacenter) &&
		s.config.ACLTokenReplication
}

func (s *Server) updateACLReplicationStatusError() {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.LastError = time.Now().Round(time.Second).UTC()
}

func (s *Server) updateACLReplicationStatusIndex(index uint64) {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.LastSuccess = time.Now().Round(time.Second).UTC()
	s.aclReplicationStatus.ReplicatedIndex = index
}

func (s *Server) updateACLReplicationStatusTokenIndex(index uint64) {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.LastSuccess = time.Now().Round(time.Second).UTC()
	s.aclReplicationStatus.ReplicatedTokenIndex = index
}

func (s *Server) initReplicationStatus() {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.Enabled = true
	s.aclReplicationStatus.Running = true
	s.aclReplicationStatus.SourceDatacenter = s.config.ACLDatacenter
}

func (s *Server) updateACLReplicationStatusStopped() {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.Running = false
}

func (s *Server) updateACLReplicationStatusRunning(replicationType structs.ACLReplicationType) {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.Running = true
	s.aclReplicationStatus.ReplicationType = replicationType
}
