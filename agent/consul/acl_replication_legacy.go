package consul

import (
	"context"
	"fmt"
	"sort"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
)

// aclIterator simplifies the algorithm below by providing a basic iterator that
// moves through a list of ACLs and returns nil when it's exhausted. It also has
// methods for pre-sorting the ACLs being iterated over by ID, which should
// already be true, but since this is crucial for correctness and we are taking
// input from other servers, we sort to make sure.
type aclIterator struct {
	acls structs.ACLs

	// index is the current position of the iterator.
	index int
}

// newACLIterator returns a new ACL iterator.
func newACLIterator(acls structs.ACLs) *aclIterator {
	return &aclIterator{acls: acls}
}

// See sort.Interface.
func (a *aclIterator) Len() int {
	return len(a.acls)
}

// See sort.Interface.
func (a *aclIterator) Swap(i, j int) {
	a.acls[i], a.acls[j] = a.acls[j], a.acls[i]
}

// See sort.Interface.
func (a *aclIterator) Less(i, j int) bool {
	return a.acls[i].ID < a.acls[j].ID
}

// Front returns the item at index position, or nil if the list is exhausted.
func (a *aclIterator) Front() *structs.ACL {
	if a.index < len(a.acls) {
		return a.acls[a.index]
	}
	return nil
}

// Next advances the iterator to the next index.
func (a *aclIterator) Next() {
	a.index++
}

// reconcileACLs takes the local and remote ACL state, and produces a list of
// changes required in order to bring the local ACLs into sync with the remote
// ACLs. You can supply lastRemoteIndex as a hint that replication has succeeded
// up to that remote index and it will make this process more efficient by only
// comparing ACL entries modified after that index. Setting this to 0 will force
// a full compare of all existing ACLs.
func reconcileLegacyACLs(local, remote structs.ACLs, lastRemoteIndex uint64) structs.ACLRequests {
	// Since sorting the lists is crucial for correctness, we are depending
	// on data coming from other servers potentially running a different,
	// version of Consul, and sorted-ness is kind of a subtle property of
	// the state store indexing, it's prudent to make sure things are sorted
	// before we begin.
	localIter, remoteIter := newACLIterator(local), newACLIterator(remote)
	sort.Sort(localIter)
	sort.Sort(remoteIter)

	// Run through both lists and reconcile them.
	var changes structs.ACLRequests
	for localIter.Front() != nil || remoteIter.Front() != nil {
		// If the local list is exhausted, then process this as a remote
		// add. We know from the loop condition that there's something
		// in the remote list.
		if localIter.Front() == nil {
			changes = append(changes, &structs.ACLRequest{
				Op:  structs.ACLSet,
				ACL: *(remoteIter.Front()),
			})
			remoteIter.Next()
			continue
		}

		// If the remote list is exhausted, then process this as a local
		// delete. We know from the loop condition that there's something
		// in the local list.
		if remoteIter.Front() == nil {
			changes = append(changes, &structs.ACLRequest{
				Op:  structs.ACLDelete,
				ACL: *(localIter.Front()),
			})
			localIter.Next()
			continue
		}

		// At this point we know there's something at the front of each
		// list we need to resolve.

		// If the remote list has something local doesn't, we add it.
		if localIter.Front().ID > remoteIter.Front().ID {
			changes = append(changes, &structs.ACLRequest{
				Op:  structs.ACLSet,
				ACL: *(remoteIter.Front()),
			})
			remoteIter.Next()
			continue
		}

		// If local has something remote doesn't, we delete it.
		if localIter.Front().ID < remoteIter.Front().ID {
			changes = append(changes, &structs.ACLRequest{
				Op:  structs.ACLDelete,
				ACL: *(localIter.Front()),
			})
			localIter.Next()
			continue
		}

		// Local and remote have an ACL with the same ID, so we might
		// need to compare them.
		l, r := localIter.Front(), remoteIter.Front()
		if r.RaftIndex.ModifyIndex > lastRemoteIndex && !r.IsSame(l) {
			changes = append(changes, &structs.ACLRequest{
				Op:  structs.ACLSet,
				ACL: *r,
			})
		}
		localIter.Next()
		remoteIter.Next()
	}
	return changes
}

// FetchLocalACLs returns the ACLs in the local state store.
func (s *Server) fetchLocalLegacyACLs() (structs.ACLs, error) {
	_, local, err := s.fsm.State().ACLTokenList(nil, false, true, "", "", "")
	if err != nil {
		return nil, err
	}

	now := time.Now()

	var acls structs.ACLs
	for _, token := range local {
		if token.IsExpired(now) {
			continue
		}
		if acl, err := token.Convert(); err == nil && acl != nil {
			acls = append(acls, acl)
		}
	}

	return acls, nil
}

// FetchRemoteACLs is used to get the remote set of ACLs from the ACL
// datacenter. The lastIndex parameter is a hint about which remote index we
// have replicated to, so this is expected to block until something changes.
func (s *Server) fetchRemoteLegacyACLs(lastRemoteIndex uint64) (*structs.IndexedACLs, error) {
	defer metrics.MeasureSince([]string{"leader", "fetchRemoteACLs"}, time.Now())

	args := structs.DCSpecificRequest{
		Datacenter: s.config.ACLDatacenter,
		QueryOptions: structs.QueryOptions{
			Token:         s.tokens.ReplicationToken(),
			MinQueryIndex: lastRemoteIndex,
			AllowStale:    true,
		},
	}
	var remote structs.IndexedACLs
	if err := s.RPC("ACL.List", &args, &remote); err != nil {
		return nil, err
	}
	return &remote, nil
}

// UpdateLocalACLs is given a list of changes to apply in order to bring the
// local ACLs in-line with the remote ACLs from the ACL datacenter.
func (s *Server) updateLocalLegacyACLs(changes structs.ACLRequests, ctx context.Context) (bool, error) {
	defer metrics.MeasureSince([]string{"leader", "updateLocalACLs"}, time.Now())

	minTimePerOp := time.Second / time.Duration(s.config.ACLReplicationApplyLimit)
	for _, change := range changes {
		// Note that we are using the single ACL interface here and not
		// performing all this inside a single transaction. This is OK
		// for two reasons. First, there's nothing else other than this
		// replication routine that alters the local ACLs, so there's
		// nothing to contend with locally. Second, if an apply fails
		// in the middle (most likely due to losing leadership), the
		// next replication pass will clean up and check everything
		// again.
		var reply string
		start := time.Now()
		if err := aclApplyInternal(s, change, &reply); err != nil {
			return false, err
		}

		// Do a smooth rate limit to wait out the min time allowed for
		// each op. If this op took longer than the min, then the sleep
		// time will be negative and we will just move on.
		elapsed := time.Since(start)
		select {
		case <-ctx.Done():
			return true, nil
		case <-time.After(minTimePerOp - elapsed):
			// do nothing
		}
	}
	return false, nil
}

// replicateACLs is a runs one pass of the algorithm for replicating ACLs from
// a remote ACL datacenter to local state. If there's any error, this will return
// 0 for the lastRemoteIndex, which will cause us to immediately do a full sync
// next time.
func (s *Server) replicateLegacyACLs(lastRemoteIndex uint64, ctx context.Context) (uint64, bool, error) {
	remote, err := s.fetchRemoteLegacyACLs(lastRemoteIndex)
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve remote ACLs: %v", err)
	}

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
	defer metrics.MeasureSince([]string{"leader", "replicateACLs"}, time.Now())

	local, err := s.fetchLocalLegacyACLs()
	if err != nil {
		return 0, false, fmt.Errorf("failed to retrieve local ACLs: %v", err)
	}

	// If the remote index ever goes backwards, it's a good indication that
	// the remote side was rebuilt and we should do a full sync since we
	// can't make any assumptions about what's going on.
	if remote.QueryMeta.Index < lastRemoteIndex {
		s.logger.Printf("[WARN] consul: Legacy ACL replication remote index moved backwards (%d to %d), forcing a full ACL sync", lastRemoteIndex, remote.QueryMeta.Index)
		lastRemoteIndex = 0
	}

	// Calculate the changes required to bring the state into sync and then
	// apply them.
	changes := reconcileLegacyACLs(local, remote.ACLs, lastRemoteIndex)
	exit, err := s.updateLocalLegacyACLs(changes, ctx)
	if exit {
		return 0, true, nil
	}

	if err != nil {
		return 0, false, fmt.Errorf("failed to sync ACL changes: %v", err)
	}

	// Return the index we got back from the remote side, since we've synced
	// up with the remote state as of that index.
	return remote.QueryMeta.Index, false, nil
}
