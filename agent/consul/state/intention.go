// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"errors"
	"fmt"
	"sort"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

const tableConnectIntentions = "connect-intentions"

// intentionsTableSchema returns a new table schema used for storing
// intentions for Connect.
func intentionsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableConnectIntentions,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.UUIDFieldIndex{
					Field: "ID",
				},
			},
			"destination": {
				Name:         "destination",
				AllowMissing: true,
				// This index is not unique since we need uniqueness across the whole
				// 4-tuple.
				Unique: false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "DestinationNS",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "DestinationName",
							Lowercase: true,
						},
					},
				},
			},
			"source": {
				Name:         "source",
				AllowMissing: true,
				// This index is not unique since we need uniqueness across the whole
				// 4-tuple.
				Unique: false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "SourceNS",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "SourceName",
							Lowercase: true,
						},
					},
				},
			},
			"source_destination": {
				Name:         "source_destination",
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "SourceNS",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "SourceName",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "DestinationNS",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "DestinationName",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
}

// LegacyIntentions is used to pull all the intentions from the snapshot.
//
// Deprecated: service-intentions config entries are handled as config entries
// in the snapshot.
func (s *Snapshot) LegacyIntentions() (structs.Intentions, error) {
	ixns, err := s.tx.Get(tableConnectIntentions, "id")
	if err != nil {
		return nil, err
	}

	var ret structs.Intentions
	for wrapped := ixns.Next(); wrapped != nil; wrapped = ixns.Next() {
		ret = append(ret, wrapped.(*structs.Intention))
	}

	return ret, nil
}

// LegacyIntention is used when restoring from a snapshot.
//
// Deprecated: service-intentions config entries are handled as config entries
// in the snapshot.
func (s *Restore) LegacyIntention(ixn *structs.Intention) error {
	// Insert the intention
	if err := s.tx.Insert(tableConnectIntentions, ixn); err != nil {
		return fmt.Errorf("failed restoring intention: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, ixn.ModifyIndex, tableConnectIntentions); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// AreIntentionsInConfigEntries determines which table is the canonical store
// for intentions data.
func (s *Store) AreIntentionsInConfigEntries() (bool, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return areIntentionsInConfigEntries(tx, nil)
}

func areIntentionsInConfigEntries(tx ReadTxn, ws memdb.WatchSet) (bool, error) {
	_, entry, err := systemMetadataGetTxn(tx, ws, structs.SystemMetadataIntentionFormatKey)
	if err != nil {
		return false, fmt.Errorf("failed system metadatalookup: %s", err)
	}
	if entry == nil {
		return false, nil
	}
	return entry.Value == structs.SystemMetadataIntentionFormatConfigValue, nil
}

// LegacyIntentions is like Intentions() but only returns legacy intentions.
// This is exposed for migration purposes.
func (s *Store) LegacyIntentions(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.Intentions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	idx, results, _, err := legacyIntentionsListTxn(tx, ws, entMeta)
	return idx, results, err
}

// Intentions returns the list of all intentions. The boolean response value is true if it came from config entries.
func (s *Store) Intentions(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.Intentions, bool, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, ws)
	if err != nil {
		return 0, nil, false, err
	}
	if !usingConfigEntries {
		return legacyIntentionsListTxn(tx, ws, entMeta)
	}
	return configIntentionsListTxn(tx, ws, entMeta)
}

func legacyIntentionsListTxn(tx ReadTxn, ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, structs.Intentions, bool, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableConnectIntentions)
	if idx < 1 {
		idx = 1
	}

	iter, err := intentionListTxn(tx, entMeta)
	if err != nil {
		return 0, nil, false, fmt.Errorf("failed intention lookup: %s", err)
	}

	ws.Add(iter.WatchCh())

	var results structs.Intentions
	for ixn := iter.Next(); ixn != nil; ixn = iter.Next() {
		results = append(results, ixn.(*structs.Intention))
	}

	// Sort by precedence just because that's nicer and probably what most clients
	// want for presentation.
	sort.Sort(structs.IntentionPrecedenceSorter(results))

	return idx, results, false, nil
}

var ErrLegacyIntentionsAreDisabled = errors.New("Legacy intention modifications are disabled after the config entry migration.")

func (s *Store) IntentionMutation(idx uint64, op structs.IntentionOp, mut *structs.IntentionMutation) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, nil)
	if err != nil {
		return err
	}
	if !usingConfigEntries {
		return errors.New("state: IntentionMutation() is not allowed when intentions are not stored in config entries")
	}

	switch op {
	case structs.IntentionOpCreate:
		if err := s.intentionMutationLegacyCreate(tx, idx, mut.Destination, mut.Value); err != nil {
			return err
		}
	case structs.IntentionOpUpdate:
		if err := s.intentionMutationLegacyUpdate(tx, idx, mut.ID, mut.Value); err != nil {
			return err
		}
	case structs.IntentionOpDelete:
		if mut.ID == "" {
			if err := s.intentionMutationDelete(tx, idx, mut.Destination, mut.Source); err != nil {
				return err
			}
		} else {
			if err := s.intentionMutationLegacyDelete(tx, idx, mut.ID); err != nil {
				return err
			}
		}
	case structs.IntentionOpUpsert:
		if err := s.intentionMutationUpsert(tx, idx, mut.Destination, mut.Source, mut.Value); err != nil {
			return err
		}
	case structs.IntentionOpDeleteAll:
		// This is an internal operation initiated by the leader and is not
		// exposed for general RPC use.
		return fmt.Errorf("Invalid Intention mutation operation '%s'", op)
	default:
		return fmt.Errorf("Invalid Intention mutation operation '%s'", op)
	}

	return tx.Commit()
}

func (s *Store) intentionMutationLegacyCreate(
	tx WriteTxn,
	idx uint64,
	dest structs.ServiceName,
	value *structs.SourceIntention,
) error {
	_, configEntry, err := configEntryTxn(tx, nil, structs.ServiceIntentions, dest.Name, &dest.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("service-intentions config entry lookup failed: %v", err)
	}

	var upsertEntry *structs.ServiceIntentionsConfigEntry
	if configEntry == nil {
		upsertEntry = &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           dest.Name,
			EnterpriseMeta: dest.EnterpriseMeta,
			Sources:        []*structs.SourceIntention{value},
		}
	} else {
		prevEntry := configEntry.(*structs.ServiceIntentionsConfigEntry)

		if err := checkLegacyIntentionApplyAllowed(prevEntry); err != nil {
			return err
		}

		upsertEntry = prevEntry.Clone()
		upsertEntry.Sources = append(upsertEntry.Sources, value)
	}

	if err := upsertEntry.LegacyNormalize(); err != nil {
		return err
	}
	if err := upsertEntry.LegacyValidate(); err != nil {
		return err
	}

	if err := ensureConfigEntryTxn(tx, idx, false, upsertEntry); err != nil {
		return err
	}

	return nil
}

func (s *Store) intentionMutationLegacyUpdate(
	tx WriteTxn,
	idx uint64,
	legacyID string,
	value *structs.SourceIntention,
) error {
	// This variant is just for legacy UUID-based intentions.

	_, prevEntry, ixn, err := s.IntentionGet(nil, legacyID)
	if err != nil {
		return fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil || prevEntry == nil {
		return fmt.Errorf("Cannot modify non-existent intention: '%s'", legacyID)
	}

	if err := checkLegacyIntentionApplyAllowed(prevEntry); err != nil {
		return err
	}

	upsertEntry := prevEntry.Clone()

	foundMatch := upsertEntry.UpdateSourceByLegacyID(
		legacyID,
		value,
	)
	if !foundMatch {
		return fmt.Errorf("Cannot modify non-existent intention: '%s'", legacyID)
	}

	if err := upsertEntry.LegacyNormalize(); err != nil {
		return err
	}
	if err := upsertEntry.LegacyValidate(); err != nil {
		return err
	}

	if err := ensureConfigEntryTxn(tx, idx, false, upsertEntry); err != nil {
		return err
	}

	return nil
}

func (s *Store) intentionMutationDelete(
	tx WriteTxn,
	idx uint64,
	dest structs.ServiceName,
	src structs.ServiceName,
) error {
	_, configEntry, err := configEntryTxn(tx, nil, structs.ServiceIntentions, dest.Name, &dest.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("service-intentions config entry lookup failed: %v", err)
	}
	if configEntry == nil {
		return nil
	}

	prevEntry := configEntry.(*structs.ServiceIntentionsConfigEntry)
	upsertEntry := prevEntry.Clone()

	deleted := upsertEntry.DeleteSourceByName(src)
	if !deleted {
		return nil
	}

	if upsertEntry == nil || len(upsertEntry.Sources) == 0 {
		return deleteConfigEntryTxn(
			tx,
			idx,
			structs.ServiceIntentions,
			dest.Name,
			&dest.EnterpriseMeta,
		)
	}

	if err := upsertEntry.Normalize(); err != nil {
		return err
	}
	if err := upsertEntry.Validate(); err != nil {
		return err
	}

	if err := ensureConfigEntryTxn(tx, idx, false, upsertEntry); err != nil {
		return err
	}

	return nil
}

func (s *Store) intentionMutationLegacyDelete(
	tx WriteTxn,
	idx uint64,
	legacyID string,
) error {
	_, prevEntry, ixn, err := s.IntentionGet(nil, legacyID)
	if err != nil {
		return fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil || prevEntry == nil {
		return fmt.Errorf("Cannot delete non-existent intention: '%s'", legacyID)
	}

	if err := checkLegacyIntentionApplyAllowed(prevEntry); err != nil {
		return err
	}

	upsertEntry := prevEntry.Clone()

	deleted := upsertEntry.DeleteSourceByLegacyID(legacyID)
	if !deleted {
		return fmt.Errorf("Cannot delete non-existent intention: '%s'", legacyID)
	}

	if upsertEntry == nil || len(upsertEntry.Sources) == 0 {
		return deleteConfigEntryTxn(
			tx,
			idx,
			structs.ServiceIntentions,
			prevEntry.Name,
			&prevEntry.EnterpriseMeta,
		)
	}

	if err := upsertEntry.LegacyNormalize(); err != nil {
		return err
	}
	if err := upsertEntry.LegacyValidate(); err != nil {
		return err
	}

	if err := ensureConfigEntryTxn(tx, idx, false, upsertEntry); err != nil {
		return err
	}

	return nil
}

func (s *Store) intentionMutationUpsert(
	tx WriteTxn,
	idx uint64,
	dest structs.ServiceName,
	src structs.ServiceName,
	value *structs.SourceIntention,
) error {
	// This variant is just for config-entry based intentions.

	_, configEntry, err := configEntryTxn(tx, nil, structs.ServiceIntentions, dest.Name, &dest.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("service-intentions config entry lookup failed: %v", err)
	}

	var prevEntry *structs.ServiceIntentionsConfigEntry
	if configEntry != nil {
		prevEntry = configEntry.(*structs.ServiceIntentionsConfigEntry)
	}

	var upsertEntry *structs.ServiceIntentionsConfigEntry

	if prevEntry == nil {
		upsertEntry = &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           dest.Name,
			EnterpriseMeta: dest.EnterpriseMeta,
			Sources:        []*structs.SourceIntention{value},
		}
	} else {
		upsertEntry = prevEntry.Clone()

		upsertEntry.UpsertSourceByName(src, value)
	}

	if err := upsertEntry.Normalize(); err != nil {
		return err
	}
	if err := upsertEntry.Validate(); err != nil {
		return err
	}

	if err := ensureConfigEntryTxn(tx, idx, false, upsertEntry); err != nil {
		return err
	}

	return nil
}

func checkLegacyIntentionApplyAllowed(prevEntry *structs.ServiceIntentionsConfigEntry) error {
	if prevEntry == nil {
		return nil
	}
	if prevEntry.LegacyIDFieldsAreAllSet() {
		return nil
	}

	sn := prevEntry.DestinationServiceName()
	return fmt.Errorf("cannot use legacy intention API to edit intentions with a destination of %q after editing them via a service-intentions config entry", sn.String())
}

// LegacyIntentionSet creates or updates an intention.
//
// Deprecated: Edit service-intentions config entries directly.
func (s *Store) LegacyIntentionSet(idx uint64, ixn *structs.Intention) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, nil)
	if err != nil {
		return err
	}
	if usingConfigEntries {
		return ErrLegacyIntentionsAreDisabled
	}

	if err := legacyIntentionSetTxn(tx, idx, ixn); err != nil {
		return err
	}

	return tx.Commit()
}

// legacyIntentionSetTxn is the inner method used to insert an intention with
// the proper indexes into the state store.
func legacyIntentionSetTxn(tx WriteTxn, idx uint64, ixn *structs.Intention) error {
	// ID is required
	if ixn.ID == "" {
		return ErrMissingIntentionID
	}

	// Ensure Precedence is populated correctly on "write"
	//nolint:staticcheck
	ixn.UpdatePrecedence()

	// Check for an existing intention
	existing, err := tx.First(tableConnectIntentions, "id", ixn.ID)
	if err != nil {
		return fmt.Errorf("failed intention lookup: %s", err)
	}
	if existing != nil {
		oldIxn := existing.(*structs.Intention)
		ixn.CreateIndex = oldIxn.CreateIndex
		ixn.CreatedAt = oldIxn.CreatedAt
	} else {
		ixn.CreateIndex = idx
	}
	ixn.ModifyIndex = idx

	// Check for duplicates on the 4-tuple.
	duplicate, err := tx.First(tableConnectIntentions, "source_destination",
		ixn.SourceNS, ixn.SourceName, ixn.DestinationNS, ixn.DestinationName)
	if err != nil {
		return fmt.Errorf("failed intention lookup: %s", err)
	}
	if duplicate != nil {
		dupIxn := duplicate.(*structs.Intention)
		// Same ID is OK - this is an update
		if dupIxn.ID != ixn.ID {
			return fmt.Errorf("duplicate intention found: %s", dupIxn.String())
		}
	}

	// We always force meta to be non-nil so that we its an empty map.
	// This makes it easy for API responses to not nil-check this everywhere.
	if ixn.Meta == nil {
		ixn.Meta = make(map[string]string)
	}

	// Insert
	if err := tx.Insert(tableConnectIntentions, ixn); err != nil {
		return err
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectIntentions, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// IntentionGet returns the given intention by ID.
func (s *Store) IntentionGet(ws memdb.WatchSet, id string) (uint64, *structs.ServiceIntentionsConfigEntry, *structs.Intention, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, ws)
	if err != nil {
		return 0, nil, nil, err
	}
	if !usingConfigEntries {
		idx, ixn, err := legacyIntentionGetTxn(tx, ws, id)
		return idx, nil, ixn, err
	}
	return configIntentionGetTxn(tx, ws, id)
}

func legacyIntentionGetTxn(tx ReadTxn, ws memdb.WatchSet, id string) (uint64, *structs.Intention, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, tableConnectIntentions)
	if idx < 1 {
		idx = 1
	}

	// Look up by its ID.
	watchCh, intention, err := tx.FirstWatch(tableConnectIntentions, "id", id)
	if err != nil {
		return 0, nil, fmt.Errorf("failed intention lookup: %s", err)
	}
	ws.Add(watchCh)

	// Convert the interface{} if it is non-nil
	var result *structs.Intention

	if intention != nil {
		result = intention.(*structs.Intention)
	}

	return idx, result, nil
}

// IntentionGetExact returns the given intention by it's full unique name.
func (s *Store) IntentionGetExact(ws memdb.WatchSet, args *structs.IntentionQueryExact) (uint64, *structs.ServiceIntentionsConfigEntry, *structs.Intention, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, ws)
	if err != nil {
		return 0, nil, nil, err
	}
	if !usingConfigEntries {
		idx, ixn, err := s.legacyIntentionGetExactTxn(tx, ws, args)
		return idx, nil, ixn, err
	}
	return s.configIntentionGetExactTxn(tx, ws, args)
}

func (s *Store) legacyIntentionGetExactTxn(tx ReadTxn, ws memdb.WatchSet, args *structs.IntentionQueryExact) (uint64, *structs.Intention, error) {
	if err := args.Validate(); err != nil {
		return 0, nil, err
	}

	// Get the table index.
	idx := maxIndexTxn(tx, tableConnectIntentions)
	if idx < 1 {
		idx = 1
	}

	// Look up by its full name.
	watchCh, intention, err := tx.FirstWatch(tableConnectIntentions, "source_destination",
		args.SourceNS, args.SourceName, args.DestinationNS, args.DestinationName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed intention lookup: %s", err)
	}
	ws.Add(watchCh)

	// Convert the interface{} if it is non-nil
	var result *structs.Intention
	if intention != nil {
		result = intention.(*structs.Intention)
	}

	return idx, result, nil
}

// LegacyIntentionDelete deletes the given intention by ID.
//
// Deprecated: Edit service-intentions config entries directly.
func (s *Store) LegacyIntentionDelete(idx uint64, id string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, nil)
	if err != nil {
		return err
	}
	if usingConfigEntries {
		return ErrLegacyIntentionsAreDisabled
	}

	if err := legacyIntentionDeleteTxn(tx, idx, id); err != nil {
		return fmt.Errorf("failed intention delete: %s", err)
	}

	return tx.Commit()
}

// legacyIntentionDeleteTxn is the inner method used to delete a legacy intention
// with the proper indexes into the state store.
func legacyIntentionDeleteTxn(tx WriteTxn, idx uint64, queryID string) error {
	// Pull the query.
	wrapped, err := tx.First(tableConnectIntentions, "id", queryID)
	if err != nil {
		return fmt.Errorf("failed intention lookup: %s", err)
	}
	if wrapped == nil {
		return nil
	}

	// Delete the query and update the index.
	if err := tx.Delete(tableConnectIntentions, wrapped); err != nil {
		return fmt.Errorf("failed intention delete: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectIntentions, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// LegacyIntentionDeleteAll deletes all legacy intentions. This is part of the
// config entry migration code.
func (s *Store) LegacyIntentionDeleteAll(idx uint64) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Delete the table and update the index.
	if _, err := tx.DeleteAll(tableConnectIntentions, "id"); err != nil {
		return fmt.Errorf("failed intention delete-all: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableConnectIntentions, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	// Also bump the index for the config entry table so that
	// secondaries can correctly know when they've replicated all of the service-intentions
	// config entries that USED to exist in the old intentions table.
	if err := tx.Insert(tableIndex, &IndexEntry{tableConfigEntries, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	// Also set a system metadata flag indicating the transition has occurred.
	metadataEntry := &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataIntentionFormatKey,
		Value: structs.SystemMetadataIntentionFormatConfigValue,
		RaftIndex: structs.RaftIndex{
			CreateIndex: idx,
			ModifyIndex: idx,
		},
	}
	if err := systemMetadataSetTxn(tx, idx, metadataEntry); err != nil {
		return fmt.Errorf("failed updating system metadata key %q: %s", metadataEntry.Key, err)
	}

	return tx.Commit()
}

type IntentionDecisionOpts struct {
	Target           string
	Namespace        string
	Partition        string
	Peer             string
	Intentions       structs.SimplifiedIntentions
	MatchType        structs.IntentionMatchType
	DefaultDecision  acl.EnforcementDecision
	AllowPermissions bool
}

// IntentionDecision returns whether a connection should be allowed to a source or destination given a set of intentions.
//
// allowPermissions determines whether the presence of L7 permissions leads to a DENY decision.
// This should be false when evaluating a connection between a source and destination, but not the request that will be sent.
func (s *Store) IntentionDecision(opts IntentionDecisionOpts) (structs.IntentionDecisionSummary, error) {

	// Figure out which source matches this request.
	var ixnMatch *structs.Intention
	for _, ixn := range opts.Intentions {
		if _, ok := connect.AuthorizeIntentionTarget(opts.Target, opts.Namespace, opts.Partition, opts.Peer, ixn, opts.MatchType); ok {
			ixnMatch = ixn
			break
		}
	}

	resp := structs.IntentionDecisionSummary{
		DefaultAllow: opts.DefaultDecision == acl.Allow,
	}
	if ixnMatch == nil {
		// No intention found, fall back to default
		resp.Allowed = resp.DefaultAllow
		return resp, nil
	}

	// Intention found, combine action + permissions
	resp.Allowed = ixnMatch.Action == structs.IntentionActionAllow
	if len(ixnMatch.Permissions) > 0 {
		// If any permissions are present, fall back to allowPermissions.
		// We are not evaluating requests so we cannot know whether the L7 permission requirements will be met.
		resp.Allowed = opts.AllowPermissions
		resp.HasPermissions = true
	}
	resp.ExternalSource = ixnMatch.Meta[structs.MetaExternalSource]

	// Intentions with wildcard namespaces but specific names are not allowed (*/web -> */api)
	// So we don't check namespaces to see if there's an exact intention
	if ixnMatch.SourceName != structs.WildcardSpecifier && ixnMatch.DestinationName != structs.WildcardSpecifier {
		resp.HasExact = true
	}

	return resp, nil
}

// IntentionMatch returns the list of intentions that match the namespace and
// name for either a source or destination. This applies the resolution rules
// so wildcards will match any value.
//
// The returned value is the list of intentions in the same order as the
// entries in args. The intentions themselves are sorted based on the
// intention precedence rules. i.e. result[0][0] is the highest precedent
// rule to match for the first entry.
func (s *Store) IntentionMatch(ws memdb.WatchSet, args *structs.IntentionQueryMatch) (uint64, []structs.Intentions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, ws)
	if err != nil {
		return 0, nil, err
	}

	if !usingConfigEntries {
		idx, ixnsList, err := s.legacyIntentionMatchTxn(tx, ws, args)
		if err != nil {
			return 0, nil, err
		}

		return idx, ixnsList, nil
	}

	maxIdx, ixnsList, err := s.configIntentionMatchTxn(tx, ws, args)
	if err != nil {
		return 0, nil, err
	}

	if args.WithSamenessGroups {
		return maxIdx, ixnsList, err
	}

	// Non-legacy intentions support sameness groups. We need to simplify them.
	var out []structs.Intentions
	for i, ixns := range ixnsList {
		entry := args.Entries[i]
		idx, simplifiedIxns, err := getSimplifiedIntentions(tx, ws, ixns)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
		}

		filteredIxns := filterIntentionsMatching(simplifiedIxns, args.Type, entry.GetEnterpriseMeta().PartitionOrDefault())

		out = append(out, filteredIxns)
	}

	return maxIdx, out, nil
}

func filterIntentionsMatching(ixns structs.Intentions, matchType structs.IntentionMatchType, partition string) structs.Intentions {
	var filteredIxns structs.Intentions
	if matchType == structs.IntentionMatchSource {
		for _, ixn := range ixns {
			if partition == ixn.SourcePartitionOrDefault() {
				filteredIxns = append(filteredIxns, ixn)
			}
		}
	} else {
		filteredIxns = ixns
	}

	return filteredIxns
}

func (s *Store) legacyIntentionMatchTxn(tx ReadTxn, ws memdb.WatchSet, args *structs.IntentionQueryMatch) (uint64, []structs.Intentions, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, tableConnectIntentions)
	if idx < 1 {
		idx = 1
	}

	// Make all the calls and accumulate the results
	results := make([]structs.Intentions, len(args.Entries))
	for i, entry := range args.Entries {
		ixns, err := intentionMatchOneTxn(tx, ws, entry, args.Type)
		if err != nil {
			return 0, nil, err
		}

		// Sort the results by precedence
		sort.Sort(structs.IntentionPrecedenceSorter(ixns))

		// Store the result
		results[i] = ixns
	}

	return idx, results, nil
}

// IntentionMatchOne returns the list of intentions that match the namespace and
// name for a single source or destination. This applies the resolution rules
// so wildcards will match any value.
//
// The returned intentions are sorted based on the intention precedence rules.
// i.e. result[0] is the highest precedent rule to match
func (s *Store) IntentionMatchOne(
	ws memdb.WatchSet,
	entry structs.IntentionMatchEntry,
	matchType structs.IntentionMatchType,
	destinationType structs.IntentionTargetType,
) (uint64, structs.SimplifiedIntentions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return compatIntentionMatchOneTxn(tx, ws, entry, matchType, destinationType)
}

func compatIntentionMatchOneTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	entry structs.IntentionMatchEntry,
	matchType structs.IntentionMatchType,
	destinationType structs.IntentionTargetType,
) (uint64, structs.SimplifiedIntentions, error) {

	usingConfigEntries, err := areIntentionsInConfigEntries(tx, ws)
	if err != nil {
		return 0, nil, err
	}
	if !usingConfigEntries {
		idx, ixns, err := legacyIntentionMatchOneTxn(tx, ws, entry, matchType)
		if err != nil {
			return 0, nil, err
		}
		return idx, structs.SimplifiedIntentions(ixns), err
	}

	maxIdx, ixns, err := configIntentionMatchOneTxn(tx, ws, entry, matchType, destinationType)
	if err != nil {
		return 0, nil, err
	}

	idx, simplifiedIxns, err := getSimplifiedIntentions(tx, ws, ixns)
	if err != nil {
		return 0, nil, err
	}

	if idx > maxIdx {
		maxIdx = idx
	}

	filteredIxns := filterIntentionsMatching(simplifiedIxns, matchType, entry.GetEnterpriseMeta().PartitionOrDefault())

	return maxIdx, structs.SimplifiedIntentions(filteredIxns), nil
}

func legacyIntentionMatchOneTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	entry structs.IntentionMatchEntry,
	matchType structs.IntentionMatchType,
) (uint64, structs.Intentions, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, tableConnectIntentions)
	if idx < 1 {
		idx = 1
	}

	results, err := intentionMatchOneTxn(tx, ws, entry, matchType)
	if err != nil {
		return 0, nil, err
	}

	sort.Sort(structs.IntentionPrecedenceSorter(results))

	return idx, results, nil
}

func intentionMatchOneTxn(tx ReadTxn, ws memdb.WatchSet,
	entry structs.IntentionMatchEntry, matchType structs.IntentionMatchType) (structs.Intentions, error) {

	// Each search entry may require multiple queries to memdb, so this
	// returns the arguments for each necessary Get. Note on performance:
	// this is not the most optimal set of queries since we repeat some
	// many times (such as */*). We can work on improving that in the
	// future, the test cases shouldn't have to change for that.
	getParams, err := intentionMatchGetParams(entry)
	if err != nil {
		return nil, err
	}

	// Perform each call and accumulate the result.
	var result structs.Intentions
	for _, params := range getParams {
		iter, err := tx.Get(tableConnectIntentions, string(matchType), params...)
		if err != nil {
			return nil, fmt.Errorf("failed intention lookup: %s", err)
		}

		ws.Add(iter.WatchCh())

		for ixn := iter.Next(); ixn != nil; ixn = iter.Next() {
			result = append(result, ixn.(*structs.Intention))
		}
	}
	return result, nil
}

// TODO(partitions): Update for partitions
// intentionMatchGetParams returns the tx.Get parameters to find all the
// intentions for a certain entry.
func intentionMatchGetParams(entry structs.IntentionMatchEntry) ([][]interface{}, error) {
	// We always query for "*/*" so include that. If the namespace is a
	// wildcard, then we're actually done.
	result := make([][]interface{}, 0, 3)
	result = append(result, []interface{}{structs.WildcardSpecifier, structs.WildcardSpecifier})
	if entry.Namespace == structs.WildcardSpecifier {
		return result, nil
	}

	// Search for NS/* intentions. If we have a wildcard name, then we're done.
	result = append(result, []interface{}{entry.Namespace, structs.WildcardSpecifier})
	if entry.Name == structs.WildcardSpecifier {
		return result, nil
	}

	// Search for the exact NS/N value.
	result = append(result, []interface{}{entry.Namespace, entry.Name})
	return result, nil
}

type ServiceWithDecision struct {
	Name     structs.ServiceName
	Decision structs.IntentionDecisionSummary
}

// IntentionTopology returns the upstreams or downstreams of a service. Upstreams and downstreams are inferred from
// intentions. If intentions allow a connection from the target to some candidate service, the candidate service is considered
// an upstream of the target.
func (s *Store) IntentionTopology(
	ws memdb.WatchSet,
	target structs.ServiceName,
	downstreams bool,
	defaultDecision acl.EnforcementDecision,
	intentionTarget structs.IntentionTargetType,
) (uint64, structs.ServiceList, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	idx, services, err := s.intentionTopologyTxn(tx, ws, target, downstreams, defaultDecision, intentionTarget)
	if err != nil {
		requested := "upstreams"
		if downstreams {
			requested = "downstreams"
		}
		return 0, nil, fmt.Errorf("failed to fetch %s for %s: %v", requested, target.String(), err)
	}

	resp := make(structs.ServiceList, 0)
	for _, svc := range services {
		resp = append(resp, structs.ServiceName{Name: svc.Name.Name, EnterpriseMeta: svc.Name.EnterpriseMeta})
	}
	return idx, resp, nil
}

func (s *Store) intentionTopologyTxn(
	tx ReadTxn, ws memdb.WatchSet,
	target structs.ServiceName,
	downstreams bool,
	defaultDecision acl.EnforcementDecision,
	intentionTarget structs.IntentionTargetType,
) (uint64, []ServiceWithDecision, error) {

	var maxIdx uint64

	// If querying the upstreams for a service, we first query intentions that apply to the target service as a source.
	// That way we can check whether intentions from the source allow connections to upstream candidates.
	// The reverse is true for downstreams.
	intentionMatchType := structs.IntentionMatchSource
	if downstreams {
		intentionMatchType = structs.IntentionMatchDestination
	}
	entry := structs.IntentionMatchEntry{
		Namespace: target.NamespaceOrDefault(),
		Partition: target.PartitionOrDefault(),
		Name:      target.Name,
	}
	index, intentions, err := compatIntentionMatchOneTxn(tx, ws, entry, intentionMatchType, intentionTarget)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to query intentions for %s", target.String())
	}
	if index > maxIdx {
		maxIdx = index
	}

	// TODO(tproxy): One remaining improvement is that this includes non-Connect services (typical services without a proxy)
	//				 Ideally those should be excluded as well, since they can't be upstreams/downstreams without a proxy.
	//				 Maybe narrow serviceNamesOfKindTxn to services represented by proxies? (ingress, sidecar-
	wildcardMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)

	services := make(map[structs.ServiceName]struct{})
	addSvcs := func(svcs []*KindServiceName) {
		for _, s := range svcs {
			services[s.Service] = struct{}{}
		}
	}

	var tempServices []*KindServiceName
	if intentionTarget == structs.IntentionTargetService {
		index, tempServices, err = serviceNamesOfKindTxn(tx, ws, structs.ServiceKindTypical, *wildcardMeta)
		if err != nil {
			return index, nil, fmt.Errorf("failed to list service names: %v", err)
		}
		addSvcs(tempServices)

		if !downstreams {
			// Query the virtual ip table as well to include virtual services that don't have a registered instance yet.
			// We only need to do this for upstreams currently, so that tproxy can find which discovery chains should be
			// contacted for failover scenarios. Virtual services technically don't need to be considered as downstreams,
			// because they will take on the identity of the calling service, rather than the chain itself.
			vipIndex, vipServices, err := servicesVirtualIPsTxn(tx, ws)
			if err != nil {
				return index, nil, fmt.Errorf("failed to list service virtual IPs: %v", err)
			}
			for _, svc := range vipServices {
				services[svc.Service.ServiceName] = struct{}{}
			}
			if vipIndex > index {
				index = vipIndex
			}
		}
	} else {
		// destinations can only ever be upstream, since they are only allowed as intention destination.
		index, tempServices, err = serviceNamesOfKindTxn(tx, ws, structs.ServiceKindDestination, *wildcardMeta)
		if err != nil {
			return index, nil, fmt.Errorf("failed to list destination service names: %v", err)
		}
		addSvcs(tempServices)
	}
	if err != nil {
		return index, nil, fmt.Errorf("failed to list ingress service names: %v", err)
	}
	if index > maxIdx {
		maxIdx = index
	}

	if downstreams {
		// Ingress gateways can only ever be downstreams, since mesh services don't dial them.
		index, ingress, err := serviceNamesOfKindTxn(tx, ws, structs.ServiceKindIngressGateway, *wildcardMeta)
		if err != nil {
			return index, nil, fmt.Errorf("failed to list ingress service names: %v", err)
		}
		if index > maxIdx {
			maxIdx = index
		}
		addSvcs(ingress)
	}

	// When checking authorization to upstreams, the match type for the decision is `destination` because we are deciding
	// if upstream candidates are covered by intentions that have the target service as a source.
	// The reverse is true for downstreams.
	decisionMatchType := structs.IntentionMatchDestination
	if downstreams {
		decisionMatchType = structs.IntentionMatchSource
	}
	result := make([]ServiceWithDecision, 0, len(services))
	for candidate := range services {
		if candidate.Name == structs.ConsulServiceName {
			continue
		}

		opts := IntentionDecisionOpts{
			Target:           candidate.Name,
			Namespace:        candidate.NamespaceOrDefault(),
			Partition:        candidate.PartitionOrDefault(),
			Intentions:       intentions,
			MatchType:        decisionMatchType,
			DefaultDecision:  defaultDecision,
			AllowPermissions: true,
		}
		decision, err := s.IntentionDecision(opts)
		if err != nil {
			src, dst := target, candidate
			if downstreams {
				src, dst = candidate, target
			}
			return 0, nil, fmt.Errorf("failed to get intention decision from (%s) to (%s): %v",
				src.String(), dst.String(), err)
		}
		if !decision.Allowed || target.Matches(candidate) {
			continue
		}

		result = append(result, ServiceWithDecision{
			Name:     structs.ServiceName{Name: candidate.Name, EnterpriseMeta: candidate.EnterpriseMeta},
			Decision: decision,
		})
	}
	return maxIdx, result, err
}
