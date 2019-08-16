package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	configTableName = "config-entries"
)

// configTableSchema returns a new table schema used to store global
// config entries.
func configTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: configTableName,
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Kind",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Name",
							Lowercase: true,
						},
					},
				},
			},
			"kind": &memdb.IndexSchema{
				Name:         "kind",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Kind",
					Lowercase: true,
				},
			},
			"link": &memdb.IndexSchema{
				Name:         "link",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ConfigEntryLinkIndex{},
			},
		},
	}
}

type ConfigEntryLinkIndex struct {
}

type discoveryChainConfigEntry interface {
	structs.ConfigEntry
	// ListRelatedServices returns a list of other names of services referenced
	// in this config entry.
	ListRelatedServices() []string
}

func (s *ConfigEntryLinkIndex) FromObject(obj interface{}) (bool, [][]byte, error) {
	entry, ok := obj.(structs.ConfigEntry)
	if !ok {
		return false, nil, fmt.Errorf("object is not a ConfigEntry")
	}

	dcEntry, ok := entry.(discoveryChainConfigEntry)
	if !ok {
		return false, nil, nil
	}

	linkedServices := dcEntry.ListRelatedServices()

	numLinks := len(linkedServices)
	if numLinks == 0 {
		return false, nil, nil
	}

	vals := make([][]byte, 0, numLinks)
	for _, linkedService := range linkedServices {
		vals = append(vals, []byte(linkedService+"\x00"))
	}

	return true, vals, nil
}

func (s *ConfigEntryLinkIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func (s *ConfigEntryLinkIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
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

func init() {
	registerSchema(configTableSchema)
}

// ConfigEntries is used to pull all the config entries for the snapshot.
func (s *Snapshot) ConfigEntries() ([]structs.ConfigEntry, error) {
	entries, err := s.tx.Get(configTableName, "id")
	if err != nil {
		return nil, err
	}

	var ret []structs.ConfigEntry
	for wrapped := entries.Next(); wrapped != nil; wrapped = entries.Next() {
		ret = append(ret, wrapped.(structs.ConfigEntry))
	}

	return ret, nil
}

// ConfigEntry is used when restoring from a snapshot.
func (s *Restore) ConfigEntry(c structs.ConfigEntry) error {
	// Insert
	if err := s.tx.Insert(configTableName, c); err != nil {
		return fmt.Errorf("failed restoring config entry object: %s", err)
	}
	if err := indexUpdateMaxTxn(s.tx, c.GetRaftIndex().ModifyIndex, configTableName); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

// ConfigEntry is called to get a given config entry.
func (s *Store) ConfigEntry(ws memdb.WatchSet, kind, name string) (uint64, structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.configEntryTxn(tx, ws, kind, name)
}

func (s *Store) configEntryTxn(tx *memdb.Txn, ws memdb.WatchSet, kind, name string) (uint64, structs.ConfigEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Get the existing config entry.
	watchCh, existing, err := tx.FirstWatch(configTableName, "id", kind, name)
	if err != nil {
		return 0, nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	ws.Add(watchCh)
	if existing == nil {
		return idx, nil, nil
	}

	conf, ok := existing.(structs.ConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("config entry %q (%s) is an invalid type: %T", name, kind, conf)
	}

	return idx, conf, nil
}

// ConfigEntries is called to get all config entry objects.
func (s *Store) ConfigEntries(ws memdb.WatchSet) (uint64, []structs.ConfigEntry, error) {
	return s.ConfigEntriesByKind(ws, "")
}

// ConfigEntriesByKind is called to get all config entry objects with the given kind.
// If kind is empty, all config entries will be returned.
func (s *Store) ConfigEntriesByKind(ws memdb.WatchSet, kind string) (uint64, []structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.configEntriesByKindTxn(tx, ws, kind)
}

func (s *Store) configEntriesByKindTxn(tx *memdb.Txn, ws memdb.WatchSet, kind string) (uint64, []structs.ConfigEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Lookup by kind, or all if kind is empty
	var iter memdb.ResultIterator
	var err error
	if kind != "" {
		iter, err = tx.Get(configTableName, "kind", kind)
	} else {
		iter, err = tx.Get(configTableName, "id")
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed config entry lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results []structs.ConfigEntry
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(structs.ConfigEntry))
	}
	return idx, results, nil
}

// EnsureConfigEntry is called to do an upsert of a given config entry.
func (s *Store) EnsureConfigEntry(idx uint64, conf structs.ConfigEntry) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	if err := s.ensureConfigEntryTxn(tx, idx, conf); err != nil {
		return err
	}

	tx.Commit()
	return nil
}

// ensureConfigEntryTxn upserts a config entry inside of a transaction.
func (s *Store) ensureConfigEntryTxn(tx *memdb.Txn, idx uint64, conf structs.ConfigEntry) error {
	// Check for existing configuration.
	existing, err := tx.First(configTableName, "id", conf.GetKind(), conf.GetName())
	if err != nil {
		return fmt.Errorf("failed configuration lookup: %s", err)
	}

	raftIndex := conf.GetRaftIndex()
	if existing != nil {
		existingIdx := existing.(structs.ConfigEntry).GetRaftIndex()
		raftIndex.CreateIndex = existingIdx.CreateIndex
		raftIndex.ModifyIndex = existingIdx.ModifyIndex
	} else {
		raftIndex.CreateIndex = idx
	}
	raftIndex.ModifyIndex = idx

	err = s.validateProposedConfigEntryInGraph(
		tx,
		idx,
		conf.GetKind(),
		conf.GetName(),
		conf,
	)
	if err != nil {
		return err // Err is already sufficiently decorated.
	}

	// Insert the config entry and update the index
	if err := tx.Insert(configTableName, conf); err != nil {
		return fmt.Errorf("failed inserting config entry: %s", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, configTableName); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	return nil
}

// EnsureConfigEntryCAS is called to do a check-and-set upsert of a given config entry.
func (s *Store) EnsureConfigEntryCAS(idx, cidx uint64, conf structs.ConfigEntry) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Check for existing configuration.
	existing, err := tx.First(configTableName, "id", conf.GetKind(), conf.GetName())
	if err != nil {
		return false, fmt.Errorf("failed configuration lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	var existingIdx structs.RaftIndex
	if existing != nil {
		existingIdx = *existing.(structs.ConfigEntry).GetRaftIndex()
	}
	if cidx == 0 && existing != nil {
		return false, nil
	}
	if cidx != 0 && existing == nil {
		return false, nil
	}
	if existing != nil && cidx != 0 && cidx != existingIdx.ModifyIndex {
		return false, nil
	}

	if err := s.ensureConfigEntryTxn(tx, idx, conf); err != nil {
		return false, err
	}

	tx.Commit()
	return true, nil
}

func (s *Store) DeleteConfigEntry(idx uint64, kind, name string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Try to retrieve the existing config entry.
	existing, err := tx.First(configTableName, "id", kind, name)
	if err != nil {
		return fmt.Errorf("failed config entry lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	err = s.validateProposedConfigEntryInGraph(
		tx,
		idx,
		kind,
		name,
		nil,
	)
	if err != nil {
		return err // Err is already sufficiently decorated.
	}

	// Delete the config entry from the DB and update the index.
	if err := tx.Delete(configTableName, existing); err != nil {
		return fmt.Errorf("failed removing check: %s", err)
	}
	if err := tx.Insert("index", &IndexEntry{configTableName, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	tx.Commit()
	return nil
}

// validateProposedConfigEntryInGraph can be used to verify graph integrity for
// a proposed graph create/update/delete.
//
// This must be called before any mutations occur on the config entries table!
//
// May return *ConfigEntryGraphValidationError if there is a concern to surface
// to the caller that they can correct.
func (s *Store) validateProposedConfigEntryInGraph(
	tx *memdb.Txn,
	idx uint64,
	kind, name string,
	next structs.ConfigEntry,
) error {
	validateAllChains := false

	switch kind {
	case structs.ProxyDefaults:
		if name != structs.ProxyConfigGlobal {
			return nil
		}
		validateAllChains = true
	case structs.ServiceDefaults:
	case structs.ServiceRouter:
	case structs.ServiceSplitter:
	case structs.ServiceResolver:
	default:
		return fmt.Errorf("unhandled kind %q during validation of %q", kind, name)
	}

	return s.validateProposedConfigEntryInServiceGraph(tx, idx, kind, name, next, validateAllChains)
}

var serviceGraphKinds = []string{
	structs.ServiceRouter,
	structs.ServiceSplitter,
	structs.ServiceResolver,
}

func (s *Store) validateProposedConfigEntryInServiceGraph(
	tx *memdb.Txn,
	idx uint64,
	kind, name string,
	next structs.ConfigEntry,
	validateAllChains bool,
) error {
	// Collect all of the chains that could be affected by this change
	// including our own.
	checkChains := make(map[string]struct{})

	if validateAllChains {
		// Must be proxy-defaults/global.

		// Check anything that has a discovery chain entry. In the future we could
		// somehow omit the ones that have a default protocol configured.

		for _, kind := range serviceGraphKinds {
			_, entries, err := s.configEntriesByKindTxn(tx, nil, kind)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				checkChains[entry.GetName()] = struct{}{}
			}
		}
	} else {
		// Must be a single chain.

		checkChains[name] = struct{}{}

		iter, err := tx.Get(configTableName, "link", name)
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			entry := raw.(structs.ConfigEntry)
			checkChains[entry.GetName()] = struct{}{}
		}
		if err != nil {
			return err
		}
	}

	overrides := map[structs.ConfigEntryKindName]structs.ConfigEntry{
		{Kind: kind, Name: name}: next,
	}

	for chainName, _ := range checkChains {
		if err := s.testCompileDiscoveryChain(tx, nil, chainName, overrides); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) testCompileDiscoveryChain(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	chainName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) error {
	_, speculativeEntries, err := s.readDiscoveryChainConfigEntriesTxn(tx, nil, chainName, overrides)
	if err != nil {
		return err
	}

	// Note we use an arbitrary namespace and datacenter as those would not
	// currently affect the graph compilation in ways that matter here.
	//
	// TODO(rb): we should thread a better value than "dc1" and the throwaway trust domain down here as that is going to sometimes show up in user facing errors
	req := discoverychain.CompileRequest{
		ServiceName:           chainName,
		EvaluateInNamespace:   "default",
		EvaluateInDatacenter:  "dc1",
		EvaluateInTrustDomain: "b6fc9da3-03d4-4b5a-9134-c045e9b20152.consul",
		UseInDatacenter:       "dc1",
		Entries:               speculativeEntries,
	}
	_, err = discoverychain.Compile(req)
	return err
}

// ReadDiscoveryChainConfigEntries will query for the full discovery chain for
// the provided service name. All relevant config entries will be recursively
// fetched and included in the result.
//
// Once returned, the caller still needs to assemble these into a useful graph
// structure.
func (s *Store) ReadDiscoveryChainConfigEntries(
	ws memdb.WatchSet,
	serviceName string,
) (uint64, *structs.DiscoveryChainConfigEntries, error) {
	return s.readDiscoveryChainConfigEntries(ws, serviceName, nil)
}

// readDiscoveryChainConfigEntries will query for the full discovery chain for
// the provided service name. All relevant config entries will be recursively
// fetched and included in the result.
//
// If 'overrides' is provided then it will use entries in that map instead of
// the database to simulate the entries that go into a modified discovery chain
// without actually modifying it yet. Nil values are tombstones to simulate
// deleting an entry.
//
// Overrides is not mutated.
func (s *Store) readDiscoveryChainConfigEntries(
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.DiscoveryChainConfigEntries, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.readDiscoveryChainConfigEntriesTxn(tx, ws, serviceName, overrides)
}

func (s *Store) readDiscoveryChainConfigEntriesTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.DiscoveryChainConfigEntries, error) {
	res := structs.NewDiscoveryChainConfigEntries()

	// Note that below we always look up splitters and resolvers in pairs, even
	// in some circumstances where both are not strictly necessary.
	//
	// For now we'll just eat the cost of fetching pairs of splitter/resolver
	// config entries even though we may not always need both. In the common
	// case we will need the pair so there's not a big drive to optimize this
	// here at this time.

	// Both Splitters and Resolvers maps will contain placeholder nils until
	// the end of this function to indicate "no such entry".

	var (
		todoSplitters = make(map[string]struct{})
		todoResolvers = make(map[string]struct{})
		todoDefaults  = make(map[string]struct{})
	)

	// Grab the proxy defaults if they exist.
	idx, proxy, err := s.getProxyConfigEntryTxn(tx, ws, structs.ProxyConfigGlobal, overrides)
	if err != nil {
		return 0, nil, err
	} else if proxy != nil {
		res.GlobalProxy = proxy
	}

	// At every step we'll need service defaults.
	todoDefaults[serviceName] = struct{}{}

	// first fetch the router, of which we only collect 1 per chain eval
	_, router, err := s.getRouterConfigEntryTxn(tx, ws, serviceName, overrides)
	if err != nil {
		return 0, nil, err
	} else if router != nil {
		res.Routers[serviceName] = router
	}

	if router != nil {
		for _, svc := range router.ListRelatedServices() {
			todoSplitters[svc] = struct{}{}
		}
	} else {
		// Next hop in the chain is the splitter.
		todoSplitters[serviceName] = struct{}{}
	}

	for {
		name, ok := anyKey(todoSplitters)
		if !ok {
			break
		}
		delete(todoSplitters, name)

		if _, ok := res.Splitters[name]; ok {
			continue // already fetched
		}

		// Yes, even for splitters.
		todoDefaults[name] = struct{}{}

		_, splitter, err := s.getSplitterConfigEntryTxn(tx, ws, name, overrides)
		if err != nil {
			return 0, nil, err
		}

		if splitter == nil {
			res.Splitters[name] = nil

			// Next hop in the chain is the resolver.
			todoResolvers[name] = struct{}{}
			continue
		}

		res.Splitters[name] = splitter

		todoResolvers[name] = struct{}{}
		for _, svc := range splitter.ListRelatedServices() {
			// If there is no splitter, this will end up adding a resolver
			// after another iteration.
			todoSplitters[svc] = struct{}{}
		}
	}

	for {
		name, ok := anyKey(todoResolvers)
		if !ok {
			break
		}
		delete(todoResolvers, name)

		if _, ok := res.Resolvers[name]; ok {
			continue // already fetched
		}

		// And resolvers, too.
		todoDefaults[name] = struct{}{}

		_, resolver, err := s.getResolverConfigEntryTxn(tx, ws, name, overrides)
		if err != nil {
			return 0, nil, err
		}

		if resolver == nil {
			res.Resolvers[name] = nil
			continue
		}

		res.Resolvers[name] = resolver

		for _, svc := range resolver.ListRelatedServices() {
			todoResolvers[svc] = struct{}{}
		}
	}

	for {
		name, ok := anyKey(todoDefaults)
		if !ok {
			break
		}
		delete(todoDefaults, name)

		if _, ok := res.Services[name]; ok {
			continue // already fetched
		}

		_, entry, err := s.getServiceConfigEntryTxn(tx, ws, name, overrides)
		if err != nil {
			return 0, nil, err
		}

		if entry == nil {
			res.Services[name] = nil
			continue
		}

		res.Services[name] = entry
	}

	// Strip nils now that they are no longer necessary.
	for name, entry := range res.Routers {
		if entry == nil {
			delete(res.Routers, name)
		}
	}
	for name, entry := range res.Splitters {
		if entry == nil {
			delete(res.Splitters, name)
		}
	}
	for name, entry := range res.Resolvers {
		if entry == nil {
			delete(res.Resolvers, name)
		}
	}
	for name, entry := range res.Services {
		if entry == nil {
			delete(res.Services, name)
		}
	}

	return idx, res, nil
}

// anyKey returns any key from the provided map if any exist. Useful for using
// a map as a simple work queue of sorts.
func anyKey(m map[string]struct{}) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	for k, _ := range m {
		return k, true
	}
	return "", false
}

// getProxyConfigEntryTxn is a convenience method for fetching a
// proxy-defaults kind of config entry.
//
// If an override is returned the index returned will be 0.
func (s *Store) getProxyConfigEntryTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	name string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.ProxyConfigEntry, error) {
	idx, entry, err := s.configEntryWithOverridesTxn(tx, ws, structs.ProxyDefaults, name, overrides)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	proxy, ok := entry.(*structs.ProxyConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, proxy, nil
}

// getServiceConfigEntryTxn is a convenience method for fetching a
// service-defaults kind of config entry.
//
// If an override is returned the index returned will be 0.
func (s *Store) getServiceConfigEntryTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.ServiceConfigEntry, error) {
	idx, entry, err := s.configEntryWithOverridesTxn(tx, ws, structs.ServiceDefaults, serviceName, overrides)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	service, ok := entry.(*structs.ServiceConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, service, nil
}

// getRouterConfigEntryTxn is a convenience method for fetching a
// service-router kind of config entry.
//
// If an override is returned the index returned will be 0.
func (s *Store) getRouterConfigEntryTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.ServiceRouterConfigEntry, error) {
	idx, entry, err := s.configEntryWithOverridesTxn(tx, ws, structs.ServiceRouter, serviceName, overrides)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	router, ok := entry.(*structs.ServiceRouterConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, router, nil
}

// getSplitterConfigEntryTxn is a convenience method for fetching a
// service-splitter kind of config entry.
//
// If an override is returned the index returned will be 0.
func (s *Store) getSplitterConfigEntryTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.ServiceSplitterConfigEntry, error) {
	idx, entry, err := s.configEntryWithOverridesTxn(tx, ws, structs.ServiceSplitter, serviceName, overrides)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	splitter, ok := entry.(*structs.ServiceSplitterConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, splitter, nil
}

// getResolverConfigEntryTxn is a convenience method for fetching a
// service-resolver kind of config entry.
//
// If an override is returned the index returned will be 0.
func (s *Store) getResolverConfigEntryTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, *structs.ServiceResolverConfigEntry, error) {
	idx, entry, err := s.configEntryWithOverridesTxn(tx, ws, structs.ServiceResolver, serviceName, overrides)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	resolver, ok := entry.(*structs.ServiceResolverConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, resolver, nil
}

func (s *Store) configEntryWithOverridesTxn(
	tx *memdb.Txn,
	ws memdb.WatchSet,
	kind string,
	name string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
) (uint64, structs.ConfigEntry, error) {
	if len(overrides) > 0 {
		entry, ok := overrides[structs.ConfigEntryKindName{
			Kind: kind, Name: name,
		}]
		if ok {
			return 0, entry, nil // a nil entry implies it should act like it is erased
		}
	}

	return s.configEntryTxn(tx, ws, kind, name)
}
