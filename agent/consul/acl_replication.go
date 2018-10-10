package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// aclBatchDeleteSize is the number of deletions to send in a single batch operation. 4096 should produce a batch that is <150KB
	// in size but should be sufficiently large to handle 1 replication round in a single batch
	aclBatchDeleteSize = 4096

	// aclBatchUpsertSize is the target size in bytes we want to submit for a batch upsert request. We estimate the size at runtime
	// due to the data being more variable in its size.
	aclBatchUpsertSize = 256 * 1024
)

func diffACLPolicies(local structs.ACLPolicies, remote structs.ACLPolicyListStubs, lastRemoteIndex uint64) ([]string, []string) {
	local.Sort()
	remote.Sort()

	var deletions []string
	var updates []string
	for localIdx, remoteIdx := 0, 0; localIdx < len(local) && remoteIdx < len(remote); {
		if local[localIdx].ID == remote[remoteIdx].ID {
			// policy is in both the local and remote state - need to check raft indices and
			if remote[remoteIdx].ModifyIndex > lastRemoteIndex && remote[remoteIdx].Hash != local[localIdx].Hash {
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

	return deletions, updates
}

func (s *Server) deleteLocalACLPolicies(deletions []string, stopCh <-chan struct{}) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	for i := 0; i < len(deletions); i += aclBatchDeleteSize {
		req := structs.ACLPolicyBatchDeleteRequest{
			PolicyIDType: structs.ACLPolicyID,
		}

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
			case <-stopCh:
				return true, nil
			case <-ticker.C:
				// do nothing - ready for the next batch
			}
		}
	}

	return false, nil
}

func (s *Server) updateLocalACLPolicies(policies structs.ACLPolicies, stopCh <-chan struct{}) (bool, error) {
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

		req := structs.ACLPolicyBatchUpsertRequest{
			Policies: policies[batchStart:batchEnd],
		}

		resp, err := s.raftApply(structs.ACLPolicyUpsertRequestType, &req)
		if err != nil {
			return false, fmt.Errorf("Failed to apply policy upserts: %v", err)
		}
		if respErr, ok := resp.(error); ok && err != nil {
			return false, fmt.Errorf("Failed to apply policy upsert: %v", respErr)
		}

		// prevent waiting if we are done
		if batchEnd < len(policies) {
			select {
			case <-stopCh:
				return true, nil
			case <-ticker.C:
				// policies[batchEnd] wasn't include as the slicing doesn't include the element at the stop index
				batchStart = batchEnd
			}
		}
	}
	return false, nil
}

func (s *Server) fetchACLPoliciesBatch(policyIDs []string) (*structs.ACLPoliciesResponse, error) {
	req := structs.ACLPolicyBatchReadRequest{
		Datacenter: s.config.ACLDatacenter,
		PolicyIDs:  policyIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ACLReplicationToken(),
		},
	}

	var response structs.ACLPoliciesResponse
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
			Token:         s.tokens.ACLReplicationToken(),
		},
	}

	var response structs.ACLPolicyListResponse
	if err := s.RPC("ACL.PolicyList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func diffACLTokens(local structs.ACLTokens, remote structs.ACLTokenListStubs, lastRemoteIndex uint64) ([]string, []string) {
	local.Sort()
	remote.Sort()

	var deletions []string
	var updates []string
	for localIdx, remoteIdx := 0, 0; localIdx < len(local) && remoteIdx < len(remote); {
		if local[localIdx].AccessorID == remote[remoteIdx].AccessorID {
			// policy is in both the local and remote state - need to check raft indices and
			if remote[remoteIdx].ModifyIndex > lastRemoteIndex && remote[remoteIdx].Hash != local[localIdx].Hash {
				updates = append(updates, remote[remoteIdx].AccessorID)
			}
			// increment both indices when equal
			localIdx += 1
			remoteIdx += 1
		} else if local[localIdx].AccessorID < remote[remoteIdx].AccessorID {
			// policy no longer in remoted state - needs deleting
			deletions = append(deletions, local[localIdx].AccessorID)

			// increment just the local index
			localIdx += 1
		} else {
			// local state doesn't have this policy - needs updating
			updates = append(updates, remote[remoteIdx].AccessorID)

			// increment just the remote index
			remoteIdx += 1
		}
	}

	return deletions, updates
}

func (s *Server) deleteLocalACLTokens(deletions []string, stopCh <-chan struct{}) (bool, error) {
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
			case <-stopCh:
				return true, nil
			case <-ticker.C:
				// do nothing - ready for the next batch
			}
		}
	}

	return false, nil
}

func (s *Server) updateLocalACLTokens(tokens structs.ACLTokens, stopCh <-chan struct{}) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	// outer loop handles submitting a batch
	for batchStart := 0; batchStart < len(tokens); {
		// inner loop finds the last element to include in this batch.
		batchSize := 0
		batchEnd := batchStart
		for ; batchEnd < len(tokens) && batchSize < aclBatchUpsertSize; batchEnd += 1 {
			batchSize += tokens[batchEnd].EstimateSize()
		}

		req := structs.ACLTokenBatchUpsertRequest{
			Tokens: tokens[batchStart:batchEnd],
		}

		resp, err := s.raftApply(structs.ACLTokenUpsertRequestType, &req)
		if err != nil {
			return false, fmt.Errorf("Failed to apply token upserts: %v", err)
		}
		if respErr, ok := resp.(error); ok && err != nil {
			return false, fmt.Errorf("Failed to apply token upserts: %v", respErr)
		}

		// prevent waiting if we are done
		if batchEnd < len(tokens) {
			select {
			case <-stopCh:
				return true, nil
			case <-ticker.C:
				// policies[batchEnd] wasn't include as the slicing doesn't include the element at the stop index
				batchStart = batchEnd
			}
		}
	}
	return false, nil
}

func (s *Server) fetchACLTokensBatch(tokenIDs []string) (*structs.ACLTokensResponse, error) {
	req := structs.ACLTokenBatchReadRequest{
		Datacenter:  s.config.ACLDatacenter,
		AccessorIDs: tokenIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ACLReplicationToken(),
		},
	}

	var response structs.ACLTokensResponse
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
			Token:         s.tokens.ACLReplicationToken(),
		},
	}

	var response structs.ACLTokenListResponse
	if err := s.RPC("ACL.TokenList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *Server) replicateACLPolicies(lastRemoteIndex uint64, stopCh <-chan struct{}) (uint64, bool, error) {
	remote, err := s.fetchACLPolicies(lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve remote ACL policies: %v", err)
	}

	// Need to check if we should be stopping. This will be common as the fetching process is a blocking
	// RPC which could have been hanging around for a long time and during that time leadership could
	// have been lost.
	select {
	case <-stopCh:
		return 0, true, nil
	default:
		// do nothing
	}

	// Measure everything after the remote query, which can block for long
	// periods of time. This metric is a good measure of how expensive the
	// replication process is.
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "policy", "apply"}, time.Now())

	_, local, err := s.fsm.State().ACLPolicyList(nil, "")
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

	// Calculate the changes required to bring the state into sync and then
	// apply them.
	deletions, updates := diffACLPolicies(local, remote.Policies, lastRemoteIndex)
	policies, err := s.fetchACLPoliciesBatch(updates)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve ACL policy updates: %v", err)
	}

	exit, err := s.deleteLocalACLPolicies(deletions, stopCh)
	if exit {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to delete local ACL policies: %v", err)
	}

	exit, err = s.updateLocalACLPolicies(policies.Policies, stopCh)
	if exit {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to update local ACL policies: %v", err)
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remote.QueryMeta.Index, false, nil
}

func (s *Server) replicateACLTokens(lastRemoteIndex uint64, stopCh <-chan struct{}) (uint64, bool, error) {
	remote, err := s.fetchACLTokens(lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve remote ACL tokens: %v", err)
	}

	// Need to check if we should be stopping. This will be common as the fetching process is a blocking
	// RPC which could have been hanging around for a long time and during that time leadership could
	// have been lost.
	select {
	case <-stopCh:
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

	// If the remote index ever goes backwards, it's a good indication that
	// the remote side was rebuilt and we should do a full sync since we
	// can't make any assumptions about what's going on.
	if remote.QueryMeta.Index < lastRemoteIndex {
		s.logger.Printf("[WARN] consul: ACL token replication remote index moved backwards (%d to %d), forcing a full ACL token sync", lastRemoteIndex, remote.QueryMeta.Index)
		lastRemoteIndex = 0
	}

	// Calculate the changes required to bring the state into sync and then
	// apply them.
	deletions, updates := diffACLTokens(local, remote.Tokens, lastRemoteIndex)
	tokens, err := s.fetchACLTokensBatch(updates)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve ACL token updates: %v", err)
	}

	exit, err := s.deleteLocalACLTokens(deletions, stopCh)
	if exit {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to delete local ACL tokens: %v", err)
	}

	exit, err = s.updateLocalACLTokens(tokens.Tokens, stopCh)
	if exit {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to update local ACL tokens: %v", err)
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remote.QueryMeta.Index, false, nil
}

// IsACLReplicationEnabled returns true if ACL replication is enabled.
func (s *Server) IsACLReplicationEnabled() bool {
	authDC := s.config.ACLDatacenter
	return len(authDC) > 0 && (authDC != s.config.Datacenter) &&
		s.config.EnableACLReplication
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
