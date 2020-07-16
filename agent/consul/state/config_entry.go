package state

import (
	"fmt"

	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	memdb "github.com/hashicorp/go-memdb"
)

const (
	configTableName = "config-entries"
)

type ConfigEntryLinkIndex struct {
}

type discoveryChainConfigEntry interface {
	structs.ConfigEntry
	// ListRelatedServices returns a list of other names of services referenced
	// in this config entry.
	ListRelatedServices() []structs.ServiceID
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
		vals = append(vals, []byte(linkedService.String()+"\x00"))
	}

	return true, vals, nil
}

func (s *ConfigEntryLinkIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(structs.ServiceID)
	if !ok {
		return nil, fmt.Errorf("argument must be a structs.ServiceID: %#v", args[0])
	}
	// Add the null character as a terminator
	return []byte(arg.String() + "\x00"), nil
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
	return insertConfigEntryWithTxn(s.tx, c.GetRaftIndex().ModifyIndex, c)
}

// ConfigEntry is called to get a given config entry.
func (s *Store) ConfigEntry(ws memdb.WatchSet, kind, name string, entMeta *structs.EnterpriseMeta) (uint64, structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return configEntryTxn(tx, ws, kind, name, entMeta)
}

func configEntryTxn(tx *txn, ws memdb.WatchSet, kind, name string, entMeta *structs.EnterpriseMeta) (uint64, structs.ConfigEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Get the existing config entry.
	watchCh, existing, err := firstWatchConfigEntryWithTxn(tx, kind, name, entMeta)
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
func (s *Store) ConfigEntries(ws memdb.WatchSet, entMeta *structs.EnterpriseMeta) (uint64, []structs.ConfigEntry, error) {
	return s.ConfigEntriesByKind(ws, "", entMeta)
}

// ConfigEntriesByKind is called to get all config entry objects with the given kind.
// If kind is empty, all config entries will be returned.
func (s *Store) ConfigEntriesByKind(ws memdb.WatchSet, kind string, entMeta *structs.EnterpriseMeta) (uint64, []structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return configEntriesByKindTxn(tx, ws, kind, entMeta)
}

func configEntriesByKindTxn(tx *txn, ws memdb.WatchSet, kind string, entMeta *structs.EnterpriseMeta) (uint64, []structs.ConfigEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, configTableName)

	// Lookup by kind, or all if kind is empty
	var iter memdb.ResultIterator
	var err error
	if kind != "" {
		iter, err = getConfigEntryKindsWithTxn(tx, kind, entMeta)
	} else {
		iter, err = getAllConfigEntriesWithTxn(tx, entMeta)
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
func (s *Store) EnsureConfigEntry(idx uint64, conf structs.ConfigEntry, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.ensureConfigEntryTxn(tx, idx, conf, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// ensureConfigEntryTxn upserts a config entry inside of a transaction.
func (s *Store) ensureConfigEntryTxn(tx *txn, idx uint64, conf structs.ConfigEntry, entMeta *structs.EnterpriseMeta) error {
	// Check for existing configuration.
	existing, err := firstConfigEntryWithTxn(tx, conf.GetKind(), conf.GetName(), entMeta)
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

	err = s.validateProposedConfigEntryInGraph(tx, conf.GetKind(), conf.GetName(), conf, entMeta)
	if err != nil {
		return err // Err is already sufficiently decorated.
	}

	if err := validateConfigEntryEnterprise(tx, conf); err != nil {
		return err
	}

	return insertConfigEntryWithTxn(tx, idx, conf)
}

// EnsureConfigEntryCAS is called to do a check-and-set upsert of a given config entry.
func (s *Store) EnsureConfigEntryCAS(idx, cidx uint64, conf structs.ConfigEntry, entMeta *structs.EnterpriseMeta) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for existing configuration.
	existing, err := firstConfigEntryWithTxn(tx, conf.GetKind(), conf.GetName(), entMeta)
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

	if err := s.ensureConfigEntryTxn(tx, idx, conf, entMeta); err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

func (s *Store) DeleteConfigEntry(idx uint64, kind, name string, entMeta *structs.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Try to retrieve the existing config entry.
	existing, err := firstConfigEntryWithTxn(tx, kind, name, entMeta)
	if err != nil {
		return fmt.Errorf("failed config entry lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// If the config entry is for terminating or ingress gateways we delete entries from the memdb table
	// that associates gateways <-> services.
	if kind == structs.TerminatingGateway || kind == structs.IngressGateway {
		if _, err := tx.DeleteAll(gatewayServicesTableName, "gateway", structs.NewServiceName(name, entMeta)); err != nil {
			return fmt.Errorf("failed to truncate gateway services table: %v", err)
		}
		if err := indexUpdateMaxTxn(tx, idx, gatewayServicesTableName); err != nil {
			return fmt.Errorf("failed updating gateway-services index: %v", err)
		}
	}

	err = s.validateProposedConfigEntryInGraph(tx, kind, name, nil, entMeta)
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

	return tx.Commit()
}

func insertConfigEntryWithTxn(tx *txn, idx uint64, conf structs.ConfigEntry) error {
	if conf == nil {
		return fmt.Errorf("cannot insert nil config entry")
	}
	// If the config entry is for a terminating or ingress gateway we update the memdb table
	// that associates gateways <-> services.
	if conf.GetKind() == structs.TerminatingGateway || conf.GetKind() == structs.IngressGateway {
		err := updateGatewayServices(tx, idx, conf, conf.GetEnterpriseMeta())
		if err != nil {
			return fmt.Errorf("failed to associate services to gateway: %v", err)
		}
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

// validateProposedConfigEntryInGraph can be used to verify graph integrity for
// a proposed graph create/update/delete.
//
// This must be called before any mutations occur on the config entries table!
//
// May return *ConfigEntryGraphValidationError if there is a concern to surface
// to the caller that they can correct.
func (s *Store) validateProposedConfigEntryInGraph(
	tx *txn,
	kind, name string,
	next structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
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
	case structs.IngressGateway:
		err := checkGatewayClash(tx, name, structs.IngressGateway, structs.TerminatingGateway, entMeta)
		if err != nil {
			return err
		}
		err = validateProposedIngressProtocolsInServiceGraph(tx, next, entMeta)
		if err != nil {
			return err
		}
	case structs.TerminatingGateway:
		err := checkGatewayClash(tx, name, structs.TerminatingGateway, structs.IngressGateway, entMeta)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unhandled kind %q during validation of %q", kind, name)
	}

	return s.validateProposedConfigEntryInServiceGraph(tx, kind, name, next, validateAllChains, entMeta)
}

func checkGatewayClash(
	tx *txn,
	name, selfKind, otherKind string,
	entMeta *structs.EnterpriseMeta,
) error {
	_, entry, err := configEntryTxn(tx, nil, otherKind, name, entMeta)
	if err != nil {
		return err
	}
	if entry != nil {
		return fmt.Errorf("cannot create a %q config entry with name %q, "+
			"a %q config entry with that name already exists", selfKind, name, otherKind)
	}
	return nil
}

var serviceGraphKinds = []string{
	structs.ServiceRouter,
	structs.ServiceSplitter,
	structs.ServiceResolver,
}

func (s *Store) validateProposedConfigEntryInServiceGraph(
	tx *txn,
	kind, name string,
	next structs.ConfigEntry,
	validateAllChains bool,
	entMeta *structs.EnterpriseMeta,
) error {
	// Collect all of the chains that could be affected by this change
	// including our own.
	checkChains := make(map[structs.ServiceID]struct{})

	if validateAllChains {
		// Must be proxy-defaults/global.

		// Check anything that has a discovery chain entry. In the future we could
		// somehow omit the ones that have a default protocol configured.

		for _, kind := range serviceGraphKinds {
			_, entries, err := configEntriesByKindTxn(tx, nil, kind, structs.WildcardEnterpriseMeta())
			if err != nil {
				return err
			}
			for _, entry := range entries {
				checkChains[structs.NewServiceID(entry.GetName(), entry.GetEnterpriseMeta())] = struct{}{}
			}
		}
	} else {
		// Must be a single chain.

		sid := structs.NewServiceID(name, entMeta)
		checkChains[sid] = struct{}{}

		iter, err := tx.Get(configTableName, "link", sid)
		if err != nil {
			return err
		}
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			entry := raw.(structs.ConfigEntry)
			checkChains[structs.NewServiceID(entry.GetName(), entry.GetEnterpriseMeta())] = struct{}{}
		}
	}

	overrides := map[structs.ConfigEntryKindName]structs.ConfigEntry{
		{Kind: kind, Name: name}: next,
	}

	for chain := range checkChains {
		if err := s.testCompileDiscoveryChain(tx, chain.ID, overrides, &chain.EnterpriseMeta); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) testCompileDiscoveryChain(
	tx *txn,
	chainName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) error {
	_, speculativeEntries, err := s.readDiscoveryChainConfigEntriesTxn(tx, nil, chainName, overrides, entMeta)
	if err != nil {
		return err
	}

	// Note we use an arbitrary namespace and datacenter as those would not
	// currently affect the graph compilation in ways that matter here.
	//
	// TODO(rb): we should thread a better value than "dc1" and the throwaway trust domain down here as that is going to sometimes show up in user facing errors
	req := discoverychain.CompileRequest{
		ServiceName:           chainName,
		EvaluateInNamespace:   entMeta.NamespaceOrDefault(),
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
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.DiscoveryChainConfigEntries, error) {
	return s.readDiscoveryChainConfigEntries(ws, serviceName, nil, entMeta)
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
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.DiscoveryChainConfigEntries, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return s.readDiscoveryChainConfigEntriesTxn(tx, ws, serviceName, overrides, entMeta)
}

func (s *Store) readDiscoveryChainConfigEntriesTxn(
	tx *txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
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
		todoSplitters = make(map[structs.ServiceID]struct{})
		todoResolvers = make(map[structs.ServiceID]struct{})
		todoDefaults  = make(map[structs.ServiceID]struct{})
	)

	sid := structs.NewServiceID(serviceName, entMeta)

	// Grab the proxy defaults if they exist.
	idx, proxy, err := s.getProxyConfigEntryTxn(tx, ws, structs.ProxyConfigGlobal, overrides, structs.DefaultEnterpriseMeta())
	if err != nil {
		return 0, nil, err
	} else if proxy != nil {
		res.GlobalProxy = proxy
	}

	// At every step we'll need service defaults.
	todoDefaults[sid] = struct{}{}

	// first fetch the router, of which we only collect 1 per chain eval
	_, router, err := s.getRouterConfigEntryTxn(tx, ws, serviceName, overrides, entMeta)
	if err != nil {
		return 0, nil, err
	} else if router != nil {
		res.Routers[sid] = router
	}

	if router != nil {
		for _, svc := range router.ListRelatedServices() {
			todoSplitters[svc] = struct{}{}
		}
	} else {
		// Next hop in the chain is the splitter.
		todoSplitters[sid] = struct{}{}
	}

	for {
		splitID, ok := anyKey(todoSplitters)
		if !ok {
			break
		}
		delete(todoSplitters, splitID)

		if _, ok := res.Splitters[splitID]; ok {
			continue // already fetched
		}

		// Yes, even for splitters.
		todoDefaults[splitID] = struct{}{}

		_, splitter, err := s.getSplitterConfigEntryTxn(tx, ws, splitID.ID, overrides, &splitID.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}

		if splitter == nil {
			res.Splitters[splitID] = nil

			// Next hop in the chain is the resolver.
			todoResolvers[splitID] = struct{}{}
			continue
		}

		res.Splitters[splitID] = splitter

		todoResolvers[splitID] = struct{}{}
		for _, svc := range splitter.ListRelatedServices() {
			// If there is no splitter, this will end up adding a resolver
			// after another iteration.
			todoSplitters[svc] = struct{}{}
		}
	}

	for {
		resolverID, ok := anyKey(todoResolvers)
		if !ok {
			break
		}
		delete(todoResolvers, resolverID)

		if _, ok := res.Resolvers[resolverID]; ok {
			continue // already fetched
		}

		// And resolvers, too.
		todoDefaults[resolverID] = struct{}{}

		_, resolver, err := s.getResolverConfigEntryTxn(tx, ws, resolverID.ID, overrides, &resolverID.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}

		if resolver == nil {
			res.Resolvers[resolverID] = nil
			continue
		}

		res.Resolvers[resolverID] = resolver

		for _, svc := range resolver.ListRelatedServices() {
			todoResolvers[svc] = struct{}{}
		}
	}

	for {
		svcID, ok := anyKey(todoDefaults)
		if !ok {
			break
		}
		delete(todoDefaults, svcID)

		if _, ok := res.Services[svcID]; ok {
			continue // already fetched
		}

		_, entry, err := s.getServiceConfigEntryTxn(tx, ws, svcID.ID, overrides, &svcID.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}

		if entry == nil {
			res.Services[svcID] = nil
			continue
		}

		res.Services[svcID] = entry
	}

	// Strip nils now that they are no longer necessary.
	for sid, entry := range res.Routers {
		if entry == nil {
			delete(res.Routers, sid)
		}
	}
	for sid, entry := range res.Splitters {
		if entry == nil {
			delete(res.Splitters, sid)
		}
	}
	for sid, entry := range res.Resolvers {
		if entry == nil {
			delete(res.Resolvers, sid)
		}
	}
	for sid, entry := range res.Services {
		if entry == nil {
			delete(res.Services, sid)
		}
	}

	return idx, res, nil
}

// anyKey returns any key from the provided map if any exist. Useful for using
// a map as a simple work queue of sorts.
func anyKey(m map[structs.ServiceID]struct{}) (structs.ServiceID, bool) {
	if len(m) == 0 {
		return structs.ServiceID{}, false
	}
	for k := range m {
		return k, true
	}
	return structs.ServiceID{}, false
}

// getProxyConfigEntryTxn is a convenience method for fetching a
// proxy-defaults kind of config entry.
//
// If an override is returned the index returned will be 0.
func (s *Store) getProxyConfigEntryTxn(
	tx *txn,
	ws memdb.WatchSet,
	name string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.ProxyConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ProxyDefaults, name, overrides, entMeta)
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
	tx *txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.ServiceConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ServiceDefaults, serviceName, overrides, entMeta)
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
	tx *txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.ServiceRouterConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ServiceRouter, serviceName, overrides, entMeta)
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
	tx *txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.ServiceSplitterConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ServiceSplitter, serviceName, overrides, entMeta)
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
	tx *txn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) (uint64, *structs.ServiceResolverConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ServiceResolver, serviceName, overrides, entMeta)
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

func configEntryWithOverridesTxn(
	tx *txn,
	ws memdb.WatchSet,
	kind string,
	name string,
	overrides map[structs.ConfigEntryKindName]structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) (uint64, structs.ConfigEntry, error) {
	if len(overrides) > 0 {
		entry, ok := overrides[structs.ConfigEntryKindName{
			Kind: kind, Name: name,
		}]
		if ok {
			return 0, entry, nil // a nil entry implies it should act like it is erased
		}
	}

	return configEntryTxn(tx, ws, kind, name, entMeta)
}

func validateProposedIngressProtocolsInServiceGraph(
	tx *txn,
	next structs.ConfigEntry,
	entMeta *structs.EnterpriseMeta,
) error {
	// This is the case for deleting a config entry
	if next == nil {
		return nil
	}
	ingress, ok := next.(*structs.IngressGatewayConfigEntry)
	if !ok {
		return fmt.Errorf("type %T is not an ingress gateway config entry", next)
	}

	validationFn := func(svc structs.ServiceName, expectedProto string) error {
		_, svcProto, err := protocolForService(tx, nil, svc)
		if err != nil {
			return err
		}

		if svcProto != expectedProto {
			return fmt.Errorf("service %q has protocol %q, which does not match defined listener protocol %q",
				svc.String(), svcProto, expectedProto)
		}

		return nil
	}

	for _, l := range ingress.Listeners {
		for _, s := range l.Services {
			if s.Name == structs.WildcardSpecifier {
				continue
			}
			err := validationFn(s.ToServiceName(), l.Protocol)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// protocolForService returns the service graph protocol associated to the
// provided service, checking all relevant config entries.
func protocolForService(
	tx *txn,
	ws memdb.WatchSet,
	svc structs.ServiceName,
) (uint64, string, error) {
	// Get the global proxy defaults (for default protocol)
	maxIdx, proxyConfig, err := configEntryTxn(tx, ws, structs.ProxyDefaults, structs.ProxyConfigGlobal, structs.DefaultEnterpriseMeta())
	if err != nil {
		return 0, "", err
	}

	idx, serviceDefaults, err := configEntryTxn(tx, ws, structs.ServiceDefaults, svc.Name, &svc.EnterpriseMeta)
	if err != nil {
		return 0, "", err
	}
	maxIdx = lib.MaxUint64(maxIdx, idx)

	entries := structs.NewDiscoveryChainConfigEntries()
	if proxyConfig != nil {
		entries.AddEntries(proxyConfig)
	}
	if serviceDefaults != nil {
		entries.AddEntries(serviceDefaults)
	}
	req := discoverychain.CompileRequest{
		ServiceName:          svc.Name,
		EvaluateInNamespace:  svc.NamespaceOrDefault(),
		EvaluateInDatacenter: "dc1",
		// Use a dummy trust domain since that won't affect the protocol here.
		EvaluateInTrustDomain: "b6fc9da3-03d4-4b5a-9134-c045e9b20152.consul",
		UseInDatacenter:       "dc1",
		Entries:               entries,
	}
	chain, err := discoverychain.Compile(req)
	if err != nil {
		return 0, "", err
	}
	return maxIdx, chain.Protocol, nil
}
