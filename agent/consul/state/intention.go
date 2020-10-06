package state

import (
	"errors"
	"fmt"
	"sort"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

const (
	intentionsTableName = "connect-intentions"
)

// intentionsTableSchema returns a new table schema used for storing
// intentions for Connect.
func intentionsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: intentionsTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name:         "id",
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

func init() {
	registerSchema(intentionsTableSchema)
}

// LegacyIntentions is used to pull all the intentions from the snapshot.
//
// Deprecated: service-intentions config entries are handled as config entries
// in the snapshot.
func (s *Snapshot) LegacyIntentions() (structs.Intentions, error) {
	ixns, err := s.tx.Get(intentionsTableName, "id")
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
	if err := s.tx.Insert(intentionsTableName, ixn); err != nil {
		return fmt.Errorf("failed restoring intention: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, ixn.ModifyIndex, intentionsTableName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// AreIntentionsInConfigEntries determines which table is the canonical store
// for intentions data.
func (s *Store) AreIntentionsInConfigEntries() (bool, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return areIntentionsInConfigEntries(tx)
}

func areIntentionsInConfigEntries(tx *txn) (bool, error) {
	_, entry, err := systemMetadataGetTxn(tx, nil, structs.SystemMetadataIntentionFormatKey)
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
func (s *Store) LegacyIntentions(ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, structs.Intentions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	idx, results, _, err := s.legacyIntentionsListTxn(tx, ws, entMeta)
	return idx, results, err
}

// Intentions returns the list of all intentions. The boolean response value is true if it came from config entries.
func (s *Store) Intentions(ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, structs.Intentions, bool, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
	if err != nil {
		return 0, nil, false, err
	}
	if !usingConfigEntries {
		return s.legacyIntentionsListTxn(tx, ws, entMeta)
	}
	return s.configIntentionsListTxn(tx, ws, entMeta)
}

func (s *Store) legacyIntentionsListTxn(tx *txn, ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, structs.Intentions, bool, error) {
	// Get the index
	idx := maxIndexTxn(tx, intentionsTableName)
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

// LegacyIntentionSet creates or updates an intention.
//
// Deprecated: Edit service-intentions config entries directly.
func (s *Store) LegacyIntentionSet(idx uint64, ixn *structs.Intention) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
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
func legacyIntentionSetTxn(tx *txn, idx uint64, ixn *structs.Intention) error {
	// ID is required
	if ixn.ID == "" {
		return ErrMissingIntentionID
	}

	// Ensure Precedence is populated correctly on "write"
	//nolint:staticcheck
	ixn.UpdatePrecedence()

	// Check for an existing intention
	existing, err := tx.First(intentionsTableName, "id", ixn.ID)
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
	duplicate, err := tx.First(intentionsTableName, "source_destination",
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
	if err := tx.Insert(intentionsTableName, ixn); err != nil {
		return err
	}
	if err := tx.Insert("index", &IndexEntry{intentionsTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// IntentionGet returns the given intention by ID.
func (s *Store) IntentionGet(ws memdb.WatchSet, id string) (uint64, *structs.ServiceIntentionsConfigEntry, *structs.Intention, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
	if err != nil {
		return 0, nil, nil, err
	}
	if !usingConfigEntries {
		idx, ixn, err := s.legacyIntentionGetTxn(tx, ws, id)
		return idx, nil, ixn, err
	}
	return s.configIntentionGetTxn(tx, ws, id)
}

func (s *Store) legacyIntentionGetTxn(tx *txn, ws memdb.WatchSet, id string) (uint64, *structs.Intention, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, intentionsTableName)
	if idx < 1 {
		idx = 1
	}

	// Look up by its ID.
	watchCh, intention, err := tx.FirstWatch(intentionsTableName, "id", id)
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

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
	if err != nil {
		return 0, nil, nil, err
	}
	if !usingConfigEntries {
		idx, ixn, err := s.legacyIntentionGetExactTxn(tx, ws, args)
		return idx, nil, ixn, err
	}
	return s.configIntentionGetExactTxn(tx, ws, args)
}

func (s *Store) legacyIntentionGetExactTxn(tx *txn, ws memdb.WatchSet, args *structs.IntentionQueryExact) (uint64, *structs.Intention, error) {
	if err := args.Validate(); err != nil {
		return 0, nil, err
	}

	// Get the table index.
	idx := maxIndexTxn(tx, intentionsTableName)
	if idx < 1 {
		idx = 1
	}

	// Look up by its full name.
	watchCh, intention, err := tx.FirstWatch(intentionsTableName, "source_destination",
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

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
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
func legacyIntentionDeleteTxn(tx *txn, idx uint64, queryID string) error {
	// Pull the query.
	wrapped, err := tx.First(intentionsTableName, "id", queryID)
	if err != nil {
		return fmt.Errorf("failed intention lookup: %s", err)
	}
	if wrapped == nil {
		return nil
	}

	// Delete the query and update the index.
	if err := tx.Delete(intentionsTableName, wrapped); err != nil {
		return fmt.Errorf("failed intention delete: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{intentionsTableName, idx}); err != nil {
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
	if _, err := tx.DeleteAll(intentionsTableName, "id"); err != nil {
		return fmt.Errorf("failed intention delete-all: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{intentionsTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	// Also bump the index for the config entry table so that
	// secondaries can correctly know when they've replicated all of the service-intentions
	// config entries that USED to exist in the old intentions table.
	if err := tx.Insert("index", &IndexEntry{configTableName, idx}); err != nil {
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

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
	if err != nil {
		return 0, nil, err
	}
	if !usingConfigEntries {
		return s.legacyIntentionMatchTxn(tx, ws, args)
	}
	return s.configIntentionMatchTxn(tx, ws, args)
}

func (s *Store) legacyIntentionMatchTxn(tx *txn, ws memdb.WatchSet, args *structs.IntentionQueryMatch) (uint64, []structs.Intentions, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, intentionsTableName)
	if idx < 1 {
		idx = 1
	}

	// Make all the calls and accumulate the results
	results := make([]structs.Intentions, len(args.Entries))
	for i, entry := range args.Entries {
		ixns, err := s.intentionMatchOneTxn(tx, ws, entry, args.Type)
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
) (uint64, structs.Intentions, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	usingConfigEntries, err := areIntentionsInConfigEntries(tx)
	if err != nil {
		return 0, nil, err
	}
	if !usingConfigEntries {
		return s.legacyIntentionMatchOneTxn(tx, ws, entry, matchType)
	}
	return s.configIntentionMatchOneTxn(tx, ws, entry, matchType)
}

func (s *Store) legacyIntentionMatchOneTxn(
	tx *txn,
	ws memdb.WatchSet,
	entry structs.IntentionMatchEntry,
	matchType structs.IntentionMatchType,
) (uint64, structs.Intentions, error) {
	// Get the table index.
	idx := maxIndexTxn(tx, intentionsTableName)
	if idx < 1 {
		idx = 1
	}

	results, err := s.intentionMatchOneTxn(tx, ws, entry, matchType)
	if err != nil {
		return 0, nil, err
	}

	sort.Sort(structs.IntentionPrecedenceSorter(results))

	return idx, results, nil
}

func (s *Store) intentionMatchOneTxn(tx ReadTxn, ws memdb.WatchSet,
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
		iter, err := tx.Get(intentionsTableName, string(matchType), params...)
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
