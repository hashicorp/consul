package consul

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	// aclReplicationMaxRetryBackoff is the max number of seconds to sleep between ACL replication RPC errors
	aclReplicationMaxRetryBackoff = 64
)

// aclTypeReplicator allows the machinery of acl replication to be shared between
// types with minimal code duplication (barring generics magically popping into
// existence).
//
// Concrete implementations of this interface should internally contain a
// pointer to the server so that data lookups can occur, and they should
// maintain the smallest quantity of type-specific state they can.
//
// Implementations of this interface are short-lived and recreated on every
// iteration.
type aclTypeReplicator interface {
	// Type is variant of replication in use. Used for updating the replication
	// status tracker.
	Type() structs.ACLReplicationType

	// SingularNoun is the singular form of the item being replicated.
	SingularNoun() string

	// PluralNoun is the plural form of the item being replicated.
	PluralNoun() string

	// FetchRemote retrieves items newer than the provided index from the
	// remote datacenter (for diffing purposes).
	FetchRemote(srv *Server, lastRemoteIndex uint64) (int, uint64, error)

	// FetchLocal retrieves items from the current datacenter (for diffing
	// purposes).
	FetchLocal(srv *Server) (int, uint64, error)

	// SortState sorts the internal working state output of FetchRemote and
	// FetchLocal so that a reasonable diff can be performed.
	SortState() (lenLocal, lenRemote int)

	// LocalMeta allows for type-agnostic metadata from the sorted local state
	// can be retrieved for the purposes of diffing.
	LocalMeta(i int) (id string, modIndex uint64, hash []byte)

	// RemoteMeta allows for type-agnostic metadata from the sorted remote
	// state can be retrieved for the purposes of diffing.
	RemoteMeta(i int) (id string, modIndex uint64, hash []byte)

	// FetchUpdated retrieves the specific items from the remote (during the
	// correction phase).
	FetchUpdated(srv *Server, updates []string) (int, error)

	// LenPendingUpdates should be the size of the data retrieved in
	// FetchUpdated.
	LenPendingUpdates() int

	// PendingUpdateIsRedacted returns true if the update contains redacted
	// data. Really only valid for tokens.
	PendingUpdateIsRedacted(i int) bool

	// PendingUpdateEstimatedSize is the item's EstimatedSize in the state
	// populated by FetchUpdated.
	PendingUpdateEstimatedSize(i int) int

	// UpdateLocalBatch applies a portion of the state populated by
	// FetchUpdated to the current datacenter.
	UpdateLocalBatch(ctx context.Context, srv *Server, start, end int) error

	// DeleteLocalBatch removes items from the current datacenter.
	DeleteLocalBatch(srv *Server, batch []string) error
}

var errContainsRedactedData = errors.New("replication results contain redacted data")

func (s *Server) fetchACLRolesBatch(roleIDs []string) (*structs.ACLRoleBatchResponse, error) {
	req := structs.ACLRoleBatchGetRequest{
		Datacenter: s.config.PrimaryDatacenter,
		RoleIDs:    roleIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ReplicationToken(),
		},
	}

	var response structs.ACLRoleBatchResponse
	if err := s.RPC(context.Background(), "ACL.RoleBatchRead", &req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Server) fetchACLRoles(lastRemoteIndex uint64) (*structs.ACLRoleListResponse, error) {
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "role", "fetch"}, time.Now())

	req := structs.ACLRoleListRequest{
		Datacenter: s.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			AllowStale:    true,
			MinQueryIndex: lastRemoteIndex,
			Token:         s.tokens.ReplicationToken(),
		},
		EnterpriseMeta: *s.replicationEnterpriseMeta(),
	}

	var response structs.ACLRoleListResponse
	if err := s.RPC(context.Background(), "ACL.RoleList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *Server) fetchACLPoliciesBatch(policyIDs []string) (*structs.ACLPolicyBatchResponse, error) {
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter: s.config.PrimaryDatacenter,
		PolicyIDs:  policyIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ReplicationToken(),
		},
	}

	var response structs.ACLPolicyBatchResponse
	if err := s.RPC(context.Background(), "ACL.PolicyBatchRead", &req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Server) fetchACLPolicies(lastRemoteIndex uint64) (*structs.ACLPolicyListResponse, error) {
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "policy", "fetch"}, time.Now())

	req := structs.ACLPolicyListRequest{
		Datacenter: s.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			AllowStale:    true,
			MinQueryIndex: lastRemoteIndex,
			Token:         s.tokens.ReplicationToken(),
		},
		EnterpriseMeta: *s.replicationEnterpriseMeta(),
	}

	var response structs.ACLPolicyListResponse
	if err := s.RPC(context.Background(), "ACL.PolicyList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

type itemDiffResults struct {
	LocalDeletes  []string
	LocalUpserts  []string
	LocalSkipped  int
	RemoteSkipped int
}

func diffACLType(tr aclTypeReplicator, lastRemoteIndex uint64) itemDiffResults {
	// Note: items with empty IDs will bubble up to the top (like legacy, unmigrated Tokens)

	lenLocal, lenRemote := tr.SortState()

	var res itemDiffResults
	var localIdx int
	var remoteIdx int
	for localIdx, remoteIdx = 0, 0; localIdx < lenLocal && remoteIdx < lenRemote; {
		localID, _, localHash := tr.LocalMeta(localIdx)
		remoteID, remoteMod, remoteHash := tr.RemoteMeta(remoteIdx)

		if localID == "" {
			res.LocalSkipped++
			localIdx += 1
			continue
		}
		if remoteID == "" {
			res.RemoteSkipped++
			remoteIdx += 1
			continue
		}

		if localID == remoteID {
			// item is in both the local and remote state - need to check raft indices and the Hash
			if remoteMod > lastRemoteIndex && !bytes.Equal(remoteHash, localHash) {
				res.LocalUpserts = append(res.LocalUpserts, remoteID)
			}
			// increment both indices when equal
			localIdx += 1
			remoteIdx += 1
		} else if localID < remoteID {
			// item no longer in remote state - needs deleting
			res.LocalDeletes = append(res.LocalDeletes, localID)

			// increment just the local index
			localIdx += 1
		} else {
			// local state doesn't have this item - needs updating
			res.LocalUpserts = append(res.LocalUpserts, remoteID)

			// increment just the remote index
			remoteIdx += 1
		}
	}

	for ; localIdx < lenLocal; localIdx += 1 {
		localID, _, _ := tr.LocalMeta(localIdx)
		if localID != "" {
			res.LocalDeletes = append(res.LocalDeletes, localID)
		} else {
			res.LocalSkipped++
		}
	}

	for ; remoteIdx < lenRemote; remoteIdx += 1 {
		remoteID, _, _ := tr.RemoteMeta(remoteIdx)
		if remoteID != "" {
			res.LocalUpserts = append(res.LocalUpserts, remoteID)
		} else {
			res.RemoteSkipped++
		}
	}

	return res
}

func (s *Server) deleteLocalACLType(ctx context.Context, tr aclTypeReplicator, deletions []string) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	for i := 0; i < len(deletions); i += aclBatchDeleteSize {
		var batch []string

		if i+aclBatchDeleteSize > len(deletions) {
			batch = deletions[i:]
		} else {
			batch = deletions[i : i+aclBatchDeleteSize]
		}

		if err := tr.DeleteLocalBatch(s, batch); err != nil {
			return false, fmt.Errorf("Failed to apply %s deletions: %v", tr.SingularNoun(), err)
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

func (s *Server) updateLocalACLType(ctx context.Context, logger hclog.Logger, tr aclTypeReplicator) (bool, error) {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.ACLReplicationApplyLimit))
	defer ticker.Stop()

	lenPending := tr.LenPendingUpdates()

	// outer loop handles submitting a batch
	for batchStart := 0; batchStart < lenPending; {
		// inner loop finds the last element to include in this batch.
		batchSize := 0
		batchEnd := batchStart
		for ; batchEnd < lenPending && batchSize < aclBatchUpsertSize; batchEnd += 1 {
			if tr.PendingUpdateIsRedacted(batchEnd) {
				return false, fmt.Errorf(
					"Detected redacted %s secrets: stopping %s update round - verify that the replication token in use has acl:write permissions.",
					tr.SingularNoun(),
					tr.SingularNoun(),
				)
			}
			batchSize += tr.PendingUpdateEstimatedSize(batchEnd)
		}

		err := tr.UpdateLocalBatch(ctx, s, batchStart, batchEnd)
		if err != nil {
			return false, fmt.Errorf("Failed to apply %s upserts: %v", tr.SingularNoun(), err)
		}
		logger.Debug(
			"acl replication - upserted batch",
			"number_upserted", batchEnd-batchStart,
			"batch_size", batchSize,
		)

		// items[batchEnd] wasn't include as the slicing doesn't include the element at the stop index
		batchStart = batchEnd

		// prevent waiting if we are done
		if batchEnd < lenPending {
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

func (s *Server) fetchACLTokensBatch(tokenAccessorIDs []string) (*structs.ACLTokenBatchResponse, error) {
	req := structs.ACLTokenBatchGetRequest{
		Datacenter:  s.config.PrimaryDatacenter,
		AccessorIDs: tokenAccessorIDs,
		QueryOptions: structs.QueryOptions{
			AllowStale: true,
			Token:      s.tokens.ReplicationToken(),
		},
	}

	var response structs.ACLTokenBatchResponse
	if err := s.RPC(context.Background(), "ACL.TokenBatchRead", &req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Server) fetchACLTokens(lastRemoteIndex uint64) (*structs.ACLTokenListResponse, error) {
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", "token", "fetch"}, time.Now())

	req := structs.ACLTokenListRequest{
		Datacenter: s.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			AllowStale:    true,
			MinQueryIndex: lastRemoteIndex,
			Token:         s.tokens.ReplicationToken(),
		},
		IncludeLocal:   false,
		IncludeGlobal:  true,
		EnterpriseMeta: *s.replicationEnterpriseMeta(),
	}

	var response structs.ACLTokenListResponse
	if err := s.RPC(context.Background(), "ACL.TokenList", &req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *Server) replicateACLTokens(ctx context.Context, logger hclog.Logger, lastRemoteIndex uint64) (uint64, bool, error) {
	tr := &aclTokenReplicator{}
	return s.replicateACLType(ctx, logger, tr, lastRemoteIndex)
}

func (s *Server) replicateACLPolicies(ctx context.Context, logger hclog.Logger, lastRemoteIndex uint64) (uint64, bool, error) {
	tr := &aclPolicyReplicator{}
	return s.replicateACLType(ctx, logger, tr, lastRemoteIndex)
}

func (s *Server) replicateACLRoles(ctx context.Context, logger hclog.Logger, lastRemoteIndex uint64) (uint64, bool, error) {
	tr := &aclRoleReplicator{}
	return s.replicateACLType(ctx, logger, tr, lastRemoteIndex)
}

func (s *Server) replicateACLType(ctx context.Context, logger hclog.Logger, tr aclTypeReplicator, lastRemoteIndex uint64) (uint64, bool, error) {
	lenRemote, remoteIndex, err := tr.FetchRemote(s, lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve remote ACL %s: %v", tr.PluralNoun(), err)
	}

	logger.Debug("finished fetching acls", "amount", lenRemote)

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
	defer metrics.MeasureSince([]string{"leader", "replication", "acl", tr.SingularNoun(), "apply"}, time.Now())

	lenLocal, _, err := tr.FetchLocal(s)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve local ACL %s: %v", tr.PluralNoun(), err)
	}

	// If the remote index ever goes backwards, it's a good indication that
	// the remote side was rebuilt and we should do a full sync since we
	// can't make any assumptions about what's going on.
	if remoteIndex < lastRemoteIndex {
		logger.Warn(
			"ACL replication remote index moved backwards, forcing a full ACL sync",
			"from", lastRemoteIndex,
			"to", remoteIndex,
		)
		lastRemoteIndex = 0
	}

	logger.Debug(
		"acl replication",
		"local", lenLocal,
		"remote", lenRemote,
	)
	// Calculate the changes required to bring the state into sync and then apply them.
	res := diffACLType(tr, lastRemoteIndex)
	if res.LocalSkipped > 0 || res.RemoteSkipped > 0 {
		logger.Debug(
			"acl replication",
			"deletions", len(res.LocalDeletes),
			"updates", len(res.LocalUpserts),
			"skipped", res.LocalSkipped,
			"skipped_remote", res.RemoteSkipped,
		)
	} else {
		logger.Debug(
			"acl replication",
			"deletions", len(res.LocalDeletes),
			"updates", len(res.LocalUpserts),
		)
	}

	if len(res.LocalUpserts) > 0 {
		lenUpdated, err := tr.FetchUpdated(s, res.LocalUpserts)
		if err == errContainsRedactedData {
			return 0, false, fmt.Errorf("failed to retrieve unredacted %s - replication token in use does not grant acl:write", tr.PluralNoun())
		} else if err != nil {
			return 0, false, fmt.Errorf("failed to retrieve ACL %s updates: %v", tr.SingularNoun(), err)
		}
		logger.Debug(
			"acl replication - downloaded updates",
			"amount", lenUpdated,
		)
	}

	if len(res.LocalDeletes) > 0 {
		logger.Debug(
			"acl replication - performing deletions",
			"amount", len(res.LocalDeletes),
		)

		exit, err := s.deleteLocalACLType(ctx, tr, res.LocalDeletes)
		if exit {
			return 0, true, nil
		}
		if err != nil {
			return 0, false, fmt.Errorf("failed to delete local ACL %s: %v", tr.PluralNoun(), err)
		}
		logger.Debug("acl replication - finished deletions")
	}

	if len(res.LocalUpserts) > 0 {
		logger.Debug("acl replication - performing updates")
		exit, err := s.updateLocalACLType(ctx, logger, tr)
		if exit {
			return 0, true, nil
		}
		if err != nil {
			return 0, false, fmt.Errorf("failed to update local ACL %s: %v", tr.PluralNoun(), err)
		}
		logger.Debug("acl replication - finished updates")
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remoteIndex, false, nil
}

func (s *Server) updateACLReplicationStatusError(errorMsg string) {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.LastError = time.Now().Round(time.Second).UTC()
	s.aclReplicationStatus.LastErrorMessage = errorMsg
}

func (s *Server) updateACLReplicationStatusIndex(replicationType structs.ACLReplicationType, index uint64) {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.LastSuccess = time.Now().Round(time.Second).UTC()
	switch replicationType {
	case structs.ACLReplicateTokens:
		s.aclReplicationStatus.ReplicatedTokenIndex = index
	case structs.ACLReplicatePolicies:
		s.aclReplicationStatus.ReplicatedIndex = index
	case structs.ACLReplicateRoles:
		s.aclReplicationStatus.ReplicatedRoleIndex = index
	default:
		panic("unknown replication type: " + replicationType.SingularNoun())
	}
}

func (s *Server) initReplicationStatus() {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.Enabled = true
	s.aclReplicationStatus.Running = true
	s.aclReplicationStatus.SourceDatacenter = s.config.PrimaryDatacenter
}

func (s *Server) updateACLReplicationStatusStopped() {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	s.aclReplicationStatus.Running = false
}

func (s *Server) updateACLReplicationStatusRunning(replicationType structs.ACLReplicationType) {
	s.aclReplicationStatusLock.Lock()
	defer s.aclReplicationStatusLock.Unlock()

	// The running state represents which type of overall replication has been
	// configured. Though there are various types of internal plumbing for acl
	// replication, to the end user there are only 3 distinctly configurable
	// variants: legacy, policy, token. Roles replicate with policies so we
	// round that up here.
	if replicationType == structs.ACLReplicateRoles {
		replicationType = structs.ACLReplicatePolicies
	}

	s.aclReplicationStatus.Running = true
	s.aclReplicationStatus.ReplicationType = replicationType
}

func (s *Server) getACLReplicationStatusRunningType() (structs.ACLReplicationType, bool) {
	s.aclReplicationStatusLock.RLock()
	defer s.aclReplicationStatusLock.RUnlock()
	return s.aclReplicationStatus.ReplicationType, s.aclReplicationStatus.Running
}

func (s *Server) getACLReplicationStatus() structs.ACLReplicationStatus {
	s.aclReplicationStatusLock.RLock()
	defer s.aclReplicationStatusLock.RUnlock()
	return s.aclReplicationStatus
}
