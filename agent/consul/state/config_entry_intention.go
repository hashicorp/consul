// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"sort"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type ServiceIntentionLegacyIDIndex struct {
	uuidFieldIndex memdb.UUIDFieldIndex // for helper code
}

func (s *ServiceIntentionLegacyIDIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	entry, ok := obj.(structs.ConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("object is not a ConfigEntry")
	}

	if entry.GetKind() != structs.ServiceIntentions {
		return false, nil, nil
	}

	ixnEntry, ok := entry.(*structs.ServiceIntentionsConfigEntry)
	if !ok {
		return false, nil, nil
	}

	// We don't pre-size this slice because it will only be populated
	// for legacy data, which should reduce over time.
	var vals [][]byte
	for _, src := range ixnEntry.Sources {
		if src.LegacyID != "" {
			arg, err := s.FromArgs(src.LegacyID)
			if err != nil {
				return false, nil, err
			}
			vals = append(vals, arg)
		}
	}

	if len(vals) == 0 {
		return false, nil, nil
	}

	return true, vals, nil
}

func (s *ServiceIntentionLegacyIDIndex) FromArgs(args ...interface{}) ([]byte, error) {
	arg, err := s.uuidFieldIndex.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Add the null character as a terminator
	b := make([]byte, 0, len(arg)+1)
	b = append(b, arg...)
	b = append(b, '\x00')
	return b, nil
}

func (s *ServiceIntentionLegacyIDIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := s.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

type SamenessGroupMemberIndex struct {
}

// Compile-time assert that these interfaces hold to ensure that the
// methods correctly exist across the ce/ent split.
var _ memdb.Indexer = (*SamenessGroupMemberIndex)(nil)
var _ memdb.MultiIndexer = (*SamenessGroupMemberIndex)(nil)

func (s *SamenessGroupMemberIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	entry, ok := obj.(structs.ConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("object is not a ConfigEntry")
	}

	sg, ok := entry.(*structs.SamenessGroupConfigEntry)
	if !ok {
		return false, nil, nil
	}

	vals := make([][]byte, 0)
	for _, m := range sg.AllMembers() {
		if m.Partition == "" {
			continue
		}

		// add 1 for null separator after each string
		buf := newIndexBuilder(len(m.Partition) + 1)
		buf.String(m.Partition)
		vals = append(vals, buf.Bytes())
	}

	if len(vals) == 0 {
		return false, nil, nil
	}

	return true, vals, nil
}

func (s *SamenessGroupMemberIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	buf := newIndexBuilder(len(arg) + 1)
	buf.String(arg)
	// Add the null character as a terminator
	return buf.Bytes(), nil
}

type ServiceIntentionSourceSamenessGroupIndex struct {
}

// Compile-time assert that these interfaces hold to ensure that the
// methods correctly exist across the ce/ent split.
var _ memdb.Indexer = (*ServiceIntentionSourceSamenessGroupIndex)(nil)
var _ memdb.MultiIndexer = (*ServiceIntentionSourceSamenessGroupIndex)(nil)

func (s *ServiceIntentionSourceSamenessGroupIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	entry, ok := obj.(structs.ConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("object is not a ConfigEntry")
	}

	ixnEntry, ok := entry.(*structs.ServiceIntentionsConfigEntry)
	if !ok {
		return false, nil, nil
	}

	vals := make([][]byte, 0, len(ixnEntry.Sources))
	for _, src := range ixnEntry.Sources {
		sg := src.SamenessGroup
		if sg == "" {
			continue
		}

		sn := structs.ServiceName{
			Name:           src.Name,
			EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(ixnEntry.PartitionOrDefault(), src.NamespaceOrDefault()),
		}.String()

		// add 2 for null separator after each string
		buf := newIndexBuilder(len(sg) + len(sn) + 2)
		buf.String(sg)
		buf.String(sn)
		vals = append(vals, buf.Bytes())
	}

	if len(vals) == 0 {
		return false, nil, nil
	}

	return true, vals, nil
}

func (s *ServiceIntentionSourceSamenessGroupIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(structs.ServiceNameWithSamenessGroup)
	if !ok {
		return nil, fmt.Errorf("argument must be a structs.ServiceID: %#v", args[0])
	}
	// Intention queries cannot use a peered service as a source
	sg := arg.SamenessGroup
	sn := arg.ServiceName.String()
	// add 2 for null separator after each string
	buf := newIndexBuilder(len(sg) + len(sn) + 2)
	buf.String(sg)
	buf.String(sn)
	// Add the null character as a terminator
	return buf.Bytes(), nil
}

type ServiceIntentionSourceIndex struct {
}

// Compile-time assert that these interfaces hold to ensure that the
// methods correctly exist across the ce/ent split.
var _ memdb.Indexer = (*ServiceIntentionSourceIndex)(nil)
var _ memdb.MultiIndexer = (*ServiceIntentionSourceIndex)(nil)

func (s *ServiceIntentionSourceIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	entry, ok := obj.(structs.ConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("object is not a ConfigEntry")
	}

	if entry.GetKind() != structs.ServiceIntentions {
		return false, nil, nil
	}

	ixnEntry, ok := entry.(*structs.ServiceIntentionsConfigEntry)
	if !ok {
		return false, nil, nil
	}

	vals := make([][]byte, 0, len(ixnEntry.Sources))
	for _, src := range ixnEntry.Sources {
		if src.SamenessGroup != "" {
			continue
		}

		peer := src.Peer
		if peer == "" {
			peer = structs.LocalPeerKeyword
		}
		sn := src.SourceServiceName().String()

		// add 2 for null separator after each string
		buf := newIndexBuilder(len(peer) + len(sn) + 2)
		buf.String(peer)
		buf.String(sn)
		vals = append(vals, buf.Bytes())
	}

	if len(vals) == 0 {
		return false, nil, nil
	}

	return true, vals, nil
}

func (s *ServiceIntentionSourceIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(structs.ServiceName)
	if !ok {
		return nil, fmt.Errorf("argument must be a structs.ServiceID: %#v", args[0])
	}
	// Intention queries cannot use a peered service as a source
	peer := structs.LocalPeerKeyword
	sn := arg.String()
	// add 2 for null separator after each string
	buf := newIndexBuilder(len(peer) + len(sn) + 2)
	buf.String(peer)
	buf.String(sn)
	// Add the null character as a terminator
	return buf.Bytes(), nil
}

func configIntentionsListTxn(tx ReadTxn, ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.Intentions, bool, error) {
	// unrolled part of configEntriesByKindTxn

	idx := maxIndexTxn(tx, tableConfigEntries)

	iter, err := getAllConfigEntriesByKindWithTxn(tx, structs.ServiceIntentions)
	if err != nil {
		return 0, nil, false, fmt.Errorf("failed config entry lookup: %s", err)
	}

	ws.Add(iter.WatchCh())

	results := configIntentionsConvertToList(iter, entMeta)

	// Sort by precedence just because that's nicer and probably what most clients
	// want for presentation.
	sort.Sort(structs.IntentionPrecedenceSorter(results))

	return idx, results, true, nil
}

func configIntentionGetTxn(tx ReadTxn, ws memdb.WatchSet, id string) (uint64, *structs.ServiceIntentionsConfigEntry, *structs.Intention, error) {
	idx := maxIndexTxn(tx, tableConfigEntries)
	if idx < 1 {
		idx = 1
	}

	watchCh, existing, err := tx.FirstWatch(tableConfigEntries, indexIntentionLegacyID, id)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	ws.Add(watchCh)
	if existing == nil {
		return idx, nil, nil, nil
	}

	conf, ok := existing.(*structs.ServiceIntentionsConfigEntry)
	if !ok {
		return 0, nil, nil, fmt.Errorf("config entry is an invalid type: %T", conf)
	}

	for _, src := range conf.Sources {
		if src.LegacyID == id {
			return idx, conf, conf.ToIntention(src), nil
		}
	}

	return idx, nil, nil, nil // Shouldn't happen.
}

func (s *Store) configIntentionGetExactTxn(tx ReadTxn, ws memdb.WatchSet, args *structs.IntentionQueryExact) (uint64, *structs.ServiceIntentionsConfigEntry, *structs.Intention, error) {
	if err := args.Validate(); err != nil {
		return 0, nil, nil, err
	}

	idx, entry, err := getServiceIntentionsConfigEntryTxn(tx, ws, args.DestinationName, nil, args.DestinationEnterpriseMeta())
	if err != nil {
		return 0, nil, nil, err
	} else if entry == nil {
		return idx, nil, nil, nil
	}

	psn := structs.PeeredServiceName{
		Peer:        args.SourcePeer,
		ServiceName: structs.NewServiceName(args.SourceName, args.SourceEnterpriseMeta()),
	}

	for _, src := range entry.Sources {
		if psn.Peer == src.Peer && psn.ServiceName == src.SourceServiceName() {
			return idx, entry, entry.ToIntention(src), nil
		}
	}

	return idx, nil, nil, nil
}

func (s *Store) configIntentionMatchTxn(tx ReadTxn, ws memdb.WatchSet, args *structs.IntentionQueryMatch) (uint64, []structs.Intentions, error) {
	maxIndex := uint64(1)

	// Make all the calls and accumulate the results
	results := make([]structs.Intentions, len(args.Entries))
	for i, entry := range args.Entries {
		// Note on performance: This is not the most optimal set of queries
		// since we repeat some many times (such as */*). We can work on
		// improving that in the future, the test cases shouldn't have to
		// change for that.

		index, ixns, err := configIntentionMatchOneTxn(tx, ws, entry, args.Type, structs.IntentionTargetService)
		if err != nil {
			return 0, nil, err
		}
		if index > maxIndex {
			maxIndex = index
		}

		// Store the result
		results[i] = ixns
	}

	return maxIndex, results, nil
}

func configIntentionMatchOneTxn(
	tx ReadTxn, ws memdb.WatchSet,
	matchEntry structs.IntentionMatchEntry,
	matchType structs.IntentionMatchType,
	targetType structs.IntentionTargetType,
) (uint64, structs.Intentions, error) {
	switch matchType {
	// targetType is only relevant for Source matches as egress Destinations can only be Intention Destinations in the mesh
	case structs.IntentionMatchSource:
		return readSourceIntentionsFromConfigEntriesTxn(tx, ws, matchEntry.Name, matchEntry.GetEnterpriseMeta(), targetType)
	case structs.IntentionMatchDestination:
		return readDestinationIntentionsFromConfigEntriesTxn(tx, ws, matchEntry.Name, matchEntry.GetEnterpriseMeta())
	default:
		return 0, nil, fmt.Errorf("invalid intention match type: %s", matchType)
	}
}

func readSourceIntentionsFromConfigEntriesTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	entMeta *acl.EnterpriseMeta,
	targetType structs.IntentionTargetType,
) (uint64, structs.Intentions, error) {
	idx := maxIndexTxn(tx, tableConfigEntries)

	var (
		results structs.Intentions
		err     error
	)

	names := getIntentionPrecedenceMatchServiceNames(serviceName, entMeta)
	for _, sn := range names {
		results, err = readSourceIntentionsFromConfigEntriesForServiceTxn(tx, ws, sn.Name, &sn.EnterpriseMeta, results, targetType)
		if err != nil {
			return 0, nil, err
		}

		results, err = readSourceSamenessIntentionsFromConfigEntriesForServiceTxn(tx, ws, sn.Name, &sn.EnterpriseMeta, results, targetType)
		if err != nil {
			return 0, nil, err
		}
	}

	// Sort the results by precedence
	sort.Sort(structs.IntentionPrecedenceSorter(results))

	return idx, results, nil
}

func readSourceIntentionsFromConfigEntriesForServiceTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	sourceEntMeta *acl.EnterpriseMeta,
	results structs.Intentions,
	targetType structs.IntentionTargetType,
) (structs.Intentions, error) {
	sn := structs.NewServiceName(serviceName, sourceEntMeta)
	iter, err := tx.Get(tableConfigEntries, indexSource, sn)
	if err != nil {
		return nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	for v := iter.Next(); v != nil; v = iter.Next() {
		entry := v.(*structs.ServiceIntentionsConfigEntry)
		entMeta := entry.DestinationServiceName().EnterpriseMeta

		kind, err := serviceIntentionsToGatewayServiceKind(tx, entry.DestinationServiceName().Name, entMeta)
		if err != nil {
			return nil, err
		}

		for _, src := range entry.Sources {
			if src.SourceServiceName() == sn {
				canAdd, err := intentionMatches(targetType, kind, entry.HasWildcardDestination())
				if err != nil {
					return nil, err
				}

				if canAdd {
					results = append(results, entry.ToIntention(src))
				}
			}
		}
	}

	return results, nil
}

func serviceIntentionsToGatewayServiceKind(tx ReadTxn, serviceName string, entMeta acl.EnterpriseMeta) (structs.GatewayServiceKind, error) {
	var err error
	kind := structs.GatewayServiceKindService

	// if we have a wildcard namespace or partition assume we are querying a service intention
	// as destination intentions will never be queried as wildcard
	if entMeta.NamespaceOrDefault() != acl.WildcardName && entMeta.PartitionOrDefault() != acl.WildcardName {
		kind, err = GatewayServiceKind(tx, serviceName, &entMeta)
		if err != nil {
			return kind, err
		}
	}

	return kind, nil
}

func intentionMatches(targetType structs.IntentionTargetType, kind structs.GatewayServiceKind, wildcardDestination bool) (bool, error) {
	var canAdd bool
	switch targetType {
	case structs.IntentionTargetService:
		canAdd = kind == structs.GatewayServiceKindService || kind == structs.GatewayServiceKindUnknown
	case structs.IntentionTargetDestination:
		// wildcard is needed here to be able to consider destinations in the wildcard intentions
		canAdd = kind == structs.GatewayServiceKindDestination || wildcardDestination
	default:
		return false, fmt.Errorf("invalid target type")
	}

	return canAdd, nil
}

func readDestinationIntentionsFromConfigEntriesTxn(tx ReadTxn, ws memdb.WatchSet, serviceName string, entMeta *acl.EnterpriseMeta) (uint64, structs.Intentions, error) {
	idx := maxIndexTxn(tx, tableConfigEntries)

	var results structs.Intentions

	names := getIntentionPrecedenceMatchServiceNames(serviceName, entMeta)
	for _, sn := range names {
		_, entry, err := getServiceIntentionsConfigEntryTxn(tx, ws, sn.Name, nil, &sn.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		} else if entry != nil {
			results = append(results, entry.ToIntentions()...)
		}
	}
	// Sort the results by precedence
	sort.Sort(structs.IntentionPrecedenceSorter(results))

	return idx, results, nil
}
