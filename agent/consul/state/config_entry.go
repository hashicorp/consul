package state

import (
	"errors"
	"fmt"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/maps"
)

type ConfigEntryLinkIndex struct {
}

type discoveryChainConfigEntry interface {
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

// ConfigEntries is used to pull all the config entries for the snapshot.
func (s *Snapshot) ConfigEntries() ([]structs.ConfigEntry, error) {
	entries, err := s.tx.Get(tableConfigEntries, "id")
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
func (s *Store) ConfigEntry(ws memdb.WatchSet, kind, name string, entMeta *acl.EnterpriseMeta) (uint64, structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return configEntryTxn(tx, ws, kind, name, entMeta)
}

func configEntryTxn(tx ReadTxn, ws memdb.WatchSet, kind, name string, entMeta *acl.EnterpriseMeta) (uint64, structs.ConfigEntry, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableConfigEntries)

	// Get the existing config entry.
	watchCh, existing, err := tx.FirstWatch(tableConfigEntries, "id", configentry.NewKindName(kind, name, entMeta))
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
func (s *Store) ConfigEntries(ws memdb.WatchSet, entMeta *acl.EnterpriseMeta) (uint64, []structs.ConfigEntry, error) {
	return s.ConfigEntriesByKind(ws, "", entMeta)
}

// ConfigEntriesByKind is called to get all config entry objects with the given kind.
// If kind is empty, all config entries will be returned.
func (s *Store) ConfigEntriesByKind(ws memdb.WatchSet, kind string, entMeta *acl.EnterpriseMeta) (uint64, []structs.ConfigEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return configEntriesByKindTxn(tx, ws, kind, entMeta)
}

func listDiscoveryChainNamesTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta acl.EnterpriseMeta,
) (uint64, []structs.ServiceName, error) {
	// Get the index and watch for updates
	idx := maxIndexWatchTxn(tx, ws, tableConfigEntries)

	// List all discovery chain top nodes.
	seen := make(map[structs.ServiceName]struct{})
	for _, kind := range []string{
		structs.ServiceRouter,
		structs.ServiceSplitter,
		structs.ServiceResolver,
	} {
		iter, err := getConfigEntryKindsWithTxn(tx, kind, &entMeta)
		if err != nil {
			return 0, nil, fmt.Errorf("failed config entry lookup: %s", err)
		}
		ws.Add(iter.WatchCh())

		for v := iter.Next(); v != nil; v = iter.Next() {
			entry := v.(structs.ConfigEntry)
			sn := structs.NewServiceName(entry.GetName(), entry.GetEnterpriseMeta())
			seen[sn] = struct{}{}
		}

		for kn, entry := range overrides {
			sn := structs.NewServiceName(kn.Name, &kn.EnterpriseMeta)
			if entry != nil {
				seen[sn] = struct{}{}
			} else {
				delete(seen, sn)
			}
		}
	}

	results := maps.SliceOfKeys(seen)
	structs.ServiceList(results).Sort()

	return idx, results, nil
}

func configEntriesByKindTxn(tx ReadTxn, ws memdb.WatchSet, kind string, entMeta *acl.EnterpriseMeta) (uint64, []structs.ConfigEntry, error) {
	// Get the index and watch for updates
	idx := maxIndexWatchTxn(tx, ws, tableConfigEntries)

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
func (s *Store) EnsureConfigEntry(idx uint64, conf structs.ConfigEntry) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := ensureConfigEntryTxn(tx, idx, false, conf); err != nil {
		return err
	}

	return tx.Commit()
}

// ensureConfigEntryTxn upserts a config entry inside of a transaction.
func ensureConfigEntryTxn(tx WriteTxn, idx uint64, statusUpdate bool, conf structs.ConfigEntry) error {
	q := newConfigEntryQuery(conf)
	existing, err := tx.First(tableConfigEntries, indexID, q)
	if err != nil {
		return fmt.Errorf("failed configuration lookup: %s", err)
	}

	raftIndex := conf.GetRaftIndex()
	if existing != nil {
		existingIdx := existing.(structs.ConfigEntry).GetRaftIndex()
		raftIndex.CreateIndex = existingIdx.CreateIndex

		// Handle optional upsert logic.
		if updatableConf, ok := conf.(structs.UpdatableConfigEntry); ok {
			if err := updatableConf.UpdateOver(existing.(structs.ConfigEntry)); err != nil {
				return err
			}
		}

		if !statusUpdate {
			if controlledConf, ok := conf.(structs.ControlledConfigEntry); ok {
				controlledConf.SetStatus(existing.(structs.ControlledConfigEntry).GetStatus())
			}
		}
	} else {
		if !statusUpdate {
			if controlledConf, ok := conf.(structs.ControlledConfigEntry); ok {
				controlledConf.SetStatus(controlledConf.DefaultStatus())
			}
		}
		raftIndex.CreateIndex = idx
	}
	raftIndex.ModifyIndex = idx

	err = validateProposedConfigEntryInGraph(tx, q, conf)
	if err != nil {
		return err // Err is already sufficiently decorated.
	}

	if err := validateConfigEntryEnterprise(tx, conf); err != nil {
		return err
	}

	return insertConfigEntryWithTxn(tx, idx, conf)
}

// EnsureConfigEntryCAS is called to do a check-and-set upsert of a given config entry.
func (s *Store) EnsureConfigEntryCAS(idx, cidx uint64, conf structs.ConfigEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for existing configuration.
	existing, err := tx.First(tableConfigEntries, indexID, newConfigEntryQuery(conf))
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

	if err := ensureConfigEntryTxn(tx, idx, false, conf); err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// EnsureConfigEntryWithStatusCAS is called to do a check-and-set upsert of a given config entry and its status.
func (s *Store) EnsureConfigEntryWithStatusCAS(idx, cidx uint64, conf structs.ConfigEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check for existing configuration.
	existing, err := tx.First(tableConfigEntries, indexID, newConfigEntryQuery(conf))
	if err != nil {
		return false, fmt.Errorf("failed configuration lookup: %s", err)
	}

	// Check if we should do the set. A ModifyIndex of 0 means that
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

	if err := ensureConfigEntryTxn(tx, idx, true, conf); err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// DeleteConfigEntryCAS performs a check-and-set deletion of a config entry
// with the given raft index. If the index is not specified, or is not equal
// to the entry's current ModifyIndex then the call is a noop, otherwise the
// normal deletion is performed.
func (s *Store) DeleteConfigEntryCAS(idx, cidx uint64, conf structs.ConfigEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := tx.First(tableConfigEntries, indexID, newConfigEntryQuery(conf))
	if err != nil {
		return false, fmt.Errorf("failed config entry lookup: %s", err)
	}

	if existing == nil {
		return false, nil
	}

	if existing.(structs.ConfigEntry).GetRaftIndex().ModifyIndex != cidx {
		return false, nil
	}

	if err := deleteConfigEntryTxn(
		tx,
		idx,
		conf.GetKind(),
		conf.GetName(),
		conf.GetEnterpriseMeta(),
	); err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

func (s *Store) DeleteConfigEntry(idx uint64, kind, name string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := deleteConfigEntryTxn(tx, idx, kind, name, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// TODO: accept structs.ConfigEntry instead of individual fields
func deleteConfigEntryTxn(tx WriteTxn, idx uint64, kind, name string, entMeta *acl.EnterpriseMeta) error {
	q := configentry.NewKindName(kind, name, entMeta)
	existing, err := tx.First(tableConfigEntries, indexID, q)
	if err != nil {
		return fmt.Errorf("failed config entry lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// If the config entry is for terminating or ingress gateways we delete entries from the memdb table
	// that associates gateways <-> services.
	sn := structs.NewServiceName(name, entMeta)

	if kind == structs.TerminatingGateway || kind == structs.IngressGateway {
		if _, err := tx.DeleteAll(tableGatewayServices, indexGateway, sn); err != nil {
			return fmt.Errorf("failed to truncate gateway services table: %v", err)
		}
		if err := indexUpdateMaxTxn(tx, idx, tableGatewayServices); err != nil {
			return fmt.Errorf("failed updating gateway-services index: %v", err)
		}
	}

	c := existing.(structs.ConfigEntry)
	switch x := c.(type) {
	case *structs.ServiceConfigEntry:
		if x.Destination != nil {
			gsKind, err := GatewayServiceKind(tx, sn.Name, &sn.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("failed to get gateway service kind for service %s: %v", sn.Name, err)
			}
			if gsKind == structs.GatewayServiceKindDestination {
				gsKind = structs.GatewayServiceKindUnknown
			}
			serviceName := structs.NewServiceName(c.GetName(), c.GetEnterpriseMeta())
			if err := checkGatewayWildcardsAndUpdate(tx, idx, &serviceName, nil, gsKind); err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}
			if err := cleanupGatewayWildcards(tx, idx, serviceName, true); err != nil {
				return fmt.Errorf("failed to cleanup gateway mapping: \"%s\"; err: %v", serviceName, err)
			}
			if err := checkGatewayAndUpdate(tx, idx, &serviceName, gsKind); err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}
			if err := cleanupKindServiceName(tx, idx, serviceName, structs.ServiceKindDestination); err != nil {
				return fmt.Errorf("failed to cleanup service name: \"%s\"; err: %v", serviceName, err)
			}
		}
	}

	// Also clean up associations in the mesh topology table for ingress gateways
	if kind == structs.IngressGateway {
		if _, err := tx.DeleteAll(tableMeshTopology, indexDownstream, sn); err != nil {
			return fmt.Errorf("failed to truncate %s table: %v", tableMeshTopology, err)
		}
		if err := indexUpdateMaxTxn(tx, idx, tableMeshTopology); err != nil {
			return fmt.Errorf("failed updating %s index: %v", tableMeshTopology, err)
		}
	}

	err = validateProposedConfigEntryInGraph(tx, q, nil)
	if err != nil {
		return err // Err is already sufficiently decorated.
	}

	// Delete the config entry from the DB and update the index.
	if err := tx.Delete(tableConfigEntries, existing); err != nil {
		return fmt.Errorf("failed removing config entry: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableConfigEntries, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func insertConfigEntryWithTxn(tx WriteTxn, idx uint64, conf structs.ConfigEntry) error {
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

	switch conf.GetKind() {
	case structs.ServiceDefaults:
		if conf.(*structs.ServiceConfigEntry).Destination != nil {
			sn := structs.ServiceName{Name: conf.GetName(), EnterpriseMeta: *conf.GetEnterpriseMeta()}
			gsKind, err := GatewayServiceKind(tx, sn.Name, &sn.EnterpriseMeta)
			if gsKind == structs.GatewayServiceKindUnknown {
				gsKind = structs.GatewayServiceKindDestination
			}
			if err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}
			if err := checkGatewayWildcardsAndUpdate(tx, idx, &sn, nil, gsKind); err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}
			if err := checkGatewayAndUpdate(tx, idx, &sn, gsKind); err != nil {
				return fmt.Errorf("failed updating gateway mapping: %s", err)
			}

			if err := upsertKindServiceName(tx, idx, structs.ServiceKindDestination, sn); err != nil {
				return fmt.Errorf("failed to persist service name: %v", err)
			}
		}
	}

	// Insert the config entry and update the index
	if err := tx.Insert(tableConfigEntries, conf); err != nil {
		return fmt.Errorf("failed inserting config entry: %s", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, tableConfigEntries); err != nil {
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
func validateProposedConfigEntryInGraph(
	tx ReadTxn,
	kindName configentry.KindName,
	newEntry structs.ConfigEntry,
) error {
	switch kindName.Kind {
	case structs.ProxyDefaults:
		// TODO: why handle an invalid case?
		if kindName.Name != structs.ProxyConfigGlobal {
			return nil
		}
	case structs.ServiceDefaults:
	case structs.ServiceRouter:
	case structs.ServiceSplitter:
	case structs.ServiceResolver:
	case structs.IngressGateway:
		err := checkGatewayClash(tx, kindName, structs.TerminatingGateway)
		if err != nil {
			return err
		}
	case structs.TerminatingGateway:
		err := checkGatewayClash(tx, kindName, structs.IngressGateway)
		if err != nil {
			return err
		}
	case structs.ServiceIntentions:
	case structs.MeshConfig:
	case structs.ExportedServices:
	case structs.APIGateway: // TODO Consider checkGatewayClash
	case structs.BoundAPIGateway:
	case structs.InlineCertificate:
	case structs.HTTPRoute:
	case structs.TCPRoute:
	default:
		return fmt.Errorf("unhandled kind %q during validation of %q", kindName.Kind, kindName.Name)
	}

	return validateProposedConfigEntryInServiceGraph(tx, kindName, newEntry)
}

func checkGatewayClash(tx ReadTxn, kindName configentry.KindName, otherKind string) error {
	_, entry, err := configEntryTxn(tx, nil, otherKind, kindName.Name, &kindName.EnterpriseMeta)
	if err != nil {
		return err
	}
	if entry != nil {
		return fmt.Errorf("cannot create a %q config entry with name %q, "+
			"a %q config entry with that name already exists", kindName.Kind, kindName.Name, otherKind)
	}
	return nil
}

var serviceGraphKinds = []string{
	structs.ServiceRouter,
	structs.ServiceSplitter,
	structs.ServiceResolver,
}

// discoveryChainTargets will return a list of services listed as a target for the input's discovery chain
func (s *Store) discoveryChainTargetsTxn(tx ReadTxn, ws memdb.WatchSet, dc, service string, entMeta *acl.EnterpriseMeta) (uint64, []structs.ServiceName, error) {
	idx, targets, err := discoveryChainOriginalTargetsTxn(tx, ws, dc, service, entMeta)
	if err != nil {
		return 0, nil, err
	}

	var resp []structs.ServiceName
	for _, t := range targets {
		em := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), t.Namespace)
		target := structs.NewServiceName(t.Service, &em)

		// TODO (freddy): Allow upstream DC and encode in response
		if t.Datacenter == dc {
			resp = append(resp, target)
		}
	}
	return idx, resp, nil
}

func discoveryChainOriginalTargetsTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	dc, service string,
	entMeta *acl.EnterpriseMeta,
) (uint64, []*structs.DiscoveryTarget, error) {
	source := structs.NewServiceName(service, entMeta)
	req := discoverychain.CompileRequest{
		ServiceName:          source.Name,
		EvaluateInNamespace:  source.NamespaceOrDefault(),
		EvaluateInPartition:  source.PartitionOrDefault(),
		EvaluateInDatacenter: dc,
	}
	idx, chain, _, err := serviceDiscoveryChainTxn(tx, ws, source.Name, entMeta, req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to fetch discovery chain for %q: %v", source.String(), err)
	}

	return idx, maps.SliceOfValues(chain.Targets), nil
}

// discoveryChainSourcesTxn will return a list of services whose discovery chains have the given service as a target
func (s *Store) discoveryChainSourcesTxn(tx ReadTxn, ws memdb.WatchSet, dc string, destination structs.ServiceName) (uint64, []structs.ServiceName, error) {
	seenLink := map[structs.ServiceName]bool{destination: true}

	queue := []structs.ServiceName{destination}
	for len(queue) > 0 {
		// The "link" index returns config entries that reference a service
		iter, err := tx.Get(tableConfigEntries, indexLink, queue[0].ToServiceID())
		if err != nil {
			return 0, nil, err
		}
		ws.Add(iter.WatchCh())

		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			entry := raw.(structs.ConfigEntry)

			sn := structs.NewServiceName(entry.GetName(), entry.GetEnterpriseMeta())
			if !seenLink[sn] {
				seenLink[sn] = true
				queue = append(queue, sn)
			}
		}
		queue = queue[1:]
	}

	var (
		maxIdx uint64 = 1
		resp   []structs.ServiceName
	)

	// Only return the services that target the destination anywhere in their discovery chains.
	seenSource := make(map[structs.ServiceName]bool)
	for sn := range seenLink {
		req := discoverychain.CompileRequest{
			ServiceName:          sn.Name,
			EvaluateInNamespace:  sn.NamespaceOrDefault(),
			EvaluateInPartition:  sn.PartitionOrDefault(),
			EvaluateInDatacenter: dc,
		}
		idx, chain, _, err := serviceDiscoveryChainTxn(tx, ws, sn.Name, &sn.EnterpriseMeta, req)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to fetch discovery chain for %q: %v", sn.String(), err)
		}

		for _, t := range chain.Targets {
			em := acl.NewEnterpriseMetaWithPartition(sn.PartitionOrDefault(), t.Namespace)
			candidate := structs.NewServiceName(t.Service, &em)

			if !candidate.Matches(destination) {
				continue
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			if !seenSource[sn] {
				seenSource[sn] = true
				resp = append(resp, sn)
			}
		}
	}
	return maxIdx, resp, nil
}

func validateProposedConfigEntryInServiceGraph(
	tx ReadTxn,
	kindName configentry.KindName,
	newEntry structs.ConfigEntry,
) error {
	// Collect all of the chains that could be affected by this change
	// including our own.
	var (
		checkChains                  = make(map[structs.ServiceID]struct{})
		checkIngress                 []*structs.IngressGatewayConfigEntry
		checkIntentions              []*structs.ServiceIntentionsConfigEntry
		enforceIngressProtocolsMatch bool
	)

	wildcardEntMeta := kindName.WithWildcardNamespace()

	switch kindName.Kind {
	case structs.ExportedServices:
		// This is the case for deleting a config entry
		if newEntry == nil {
			return nil
		}

		entry := newEntry.(*structs.ExportedServicesConfigEntry)

		_, serviceList, err := listServicesExportedToAnyPeerByConfigEntry(nil, tx, entry, nil)
		if err != nil {
			return err
		}

		for _, sn := range serviceList {
			if err := validateChainIsPeerExportSafe(tx, sn, nil); err != nil {
				return err
			}
		}

		return nil

	case structs.MeshConfig:
		// Exported services and mesh config do not influence discovery chains.
		return nil

	case structs.ProxyDefaults:
		// Check anything that has a discovery chain entry. In the future we could
		// somehow omit the ones that have a default protocol configured.

		for _, kind := range serviceGraphKinds {
			_, entries, err := configEntriesByKindTxn(tx, nil, kind, wildcardEntMeta)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				checkChains[structs.NewServiceID(entry.GetName(), entry.GetEnterpriseMeta())] = struct{}{}
			}
		}

		_, ingressEntries, err := configEntriesByKindTxn(tx, nil, structs.IngressGateway, wildcardEntMeta)
		if err != nil {
			return err
		}
		for _, entry := range ingressEntries {
			ingress, ok := entry.(*structs.IngressGatewayConfigEntry)
			if !ok {
				return fmt.Errorf("type %T is not an ingress gateway config entry", entry)
			}
			checkIngress = append(checkIngress, ingress)
		}

		_, ixnEntries, err := configEntriesByKindTxn(tx, nil, structs.ServiceIntentions, wildcardEntMeta)
		if err != nil {
			return err
		}
		for _, entry := range ixnEntries {
			ixn, ok := entry.(*structs.ServiceIntentionsConfigEntry)
			if !ok {
				return fmt.Errorf("type %T is not a service intentions config entry", entry)
			}
			checkIntentions = append(checkIntentions, ixn)
		}

	case structs.ServiceIntentions:
		// Check that the protocols match.

		// This is the case for deleting a config entry
		if newEntry == nil {
			return nil
		}

		ixn, ok := newEntry.(*structs.ServiceIntentionsConfigEntry)
		if !ok {
			return fmt.Errorf("type %T is not a service intentions config entry", newEntry)
		}
		checkIntentions = append(checkIntentions, ixn)

	case structs.IngressGateway:
		// Checking an ingress pointing to multiple chains.

		// This is the case for deleting a config entry
		if newEntry == nil {
			return nil
		}

		ingress, ok := newEntry.(*structs.IngressGatewayConfigEntry)
		if !ok {
			return fmt.Errorf("type %T is not an ingress gateway config entry", newEntry)
		}
		checkIngress = append(checkIngress, ingress)

		// When editing an ingress-gateway directly we are stricter about
		// validating the protocol equivalence.
		enforceIngressProtocolsMatch = true

	default:
		// Must be a single chain.

		// Check to see if we should ensure L7 intentions have an L7 protocol.
		_, ixn, err := getServiceIntentionsConfigEntryTxn(
			tx, nil, kindName.Name, nil, &kindName.EnterpriseMeta,
		)
		if err != nil {
			return err
		} else if ixn != nil {
			checkIntentions = append(checkIntentions, ixn)
		}

		_, ixnEntries, err := configEntriesByKindTxn(tx, nil, structs.ServiceIntentions, wildcardEntMeta)
		if err != nil {
			return err
		}
		for _, entry := range ixnEntries {
			ixn, ok := entry.(*structs.ServiceIntentionsConfigEntry)
			if !ok {
				return fmt.Errorf("type %T is not a service intentions config entry", entry)
			}
			checkIntentions = append(checkIntentions, ixn)
		}

		sid := structs.NewServiceID(kindName.Name, &kindName.EnterpriseMeta)
		checkChains[sid] = struct{}{}

		iter, err := tx.Get(tableConfigEntries, indexLink, sid)
		if err != nil {
			return err
		}
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			entry := raw.(structs.ConfigEntry)
			switch entry.GetKind() {
			case structs.ServiceRouter, structs.ServiceSplitter, structs.ServiceResolver:
				svcID := structs.NewServiceID(entry.GetName(), entry.GetEnterpriseMeta())
				checkChains[svcID] = struct{}{}
			case structs.IngressGateway:
				ingress, ok := entry.(*structs.IngressGatewayConfigEntry)
				if !ok {
					return fmt.Errorf("type %T is not an ingress gateway config entry", entry)
				}
				checkIngress = append(checkIngress, ingress)
			}
		}
	}

	// Ensure if any ingress or intention is affected that we fetch all of the
	// chains needed to fully validate them.
	for _, ingress := range checkIngress {
		for _, svcID := range ingress.ListRelatedServices() {
			checkChains[svcID] = struct{}{}
		}
	}
	for _, ixn := range checkIntentions {
		sn := ixn.DestinationServiceName()
		checkChains[sn.ToServiceID()] = struct{}{}
	}

	overrides := map[configentry.KindName]structs.ConfigEntry{
		kindName: newEntry,
	}

	var (
		svcProtocols                = make(map[structs.ServiceID]string)
		svcTopNodeType              = make(map[structs.ServiceID]string)
		exportedServicesByPartition = make(map[string]map[structs.ServiceName]struct{})
	)
	for chain := range checkChains {
		protocol, topNode, newTargets, err := testCompileDiscoveryChain(tx, chain.ID, overrides, &chain.EnterpriseMeta)
		if err != nil {
			return err
		}
		svcProtocols[chain] = protocol
		svcTopNodeType[chain] = topNode.Type

		chainSvc := structs.NewServiceName(chain.ID, &chain.EnterpriseMeta)

		// Validate that we aren't adding a cross-datacenter or cross-partition
		// reference to a peer-exported service's discovery chain by this pending
		// edit.
		partition := chain.PartitionOrDefault()
		exportedServices, ok := exportedServicesByPartition[partition]
		if !ok {
			entMeta := structs.NodeEnterpriseMetaInPartition(partition)
			_, exportedServices, err = listAllExportedServices(nil, tx, overrides, *entMeta)
			if err != nil {
				return err
			}
			exportedServicesByPartition[partition] = exportedServices
		}
		if _, exported := exportedServices[chainSvc]; exported {
			if err := validateChainIsPeerExportSafe(tx, chainSvc, overrides); err != nil {
				return err
			}

			// If a TCP (L4) discovery chain is peer exported we have to take
			// care to prohibit certain edits to service-resolvers.
			if !structs.IsProtocolHTTPLike(protocol) {
				_, _, oldTargets, err := testCompileDiscoveryChain(tx, chain.ID, nil, &chain.EnterpriseMeta)
				if err != nil {
					return fmt.Errorf("error compiling current discovery chain for %q: %w", chainSvc, err)
				}

				// Ensure that you can't introduce any new targets that would
				// produce a new SpiffeID for this L4 service.
				oldSpiffeIDs := convertTargetsToTestSpiffeIDs(oldTargets)
				newSpiffeIDs := convertTargetsToTestSpiffeIDs(newTargets)
				for id, targetID := range newSpiffeIDs {
					if _, exists := oldSpiffeIDs[id]; !exists {
						return fmt.Errorf("peer exported service %q uses protocol=%q and cannot introduce new discovery chain targets like %q",
							chainSvc, protocol, targetID,
						)
					}
				}
			}
		}
	}

	// Now validate all of our ingress gateways.
	for _, e := range checkIngress {
		for _, listener := range e.Listeners {
			expectedProto := listener.Protocol
			for _, service := range listener.Services {
				if service.Name == structs.WildcardSpecifier {
					continue
				}
				svcID := structs.NewServiceID(service.Name, &service.EnterpriseMeta)

				svcProto := svcProtocols[svcID]

				if svcProto != expectedProto {
					// The only time an ingress gateway and its upstreams can
					// have differing protocols is when:
					//
					// 1. ingress is tcp and the target is not-tcp
					//    AND
					// 2. the disco chain has a resolver as the top node
					topNodeType := svcTopNodeType[svcID]
					if enforceIngressProtocolsMatch ||
						(expectedProto != "tcp") ||
						(expectedProto == "tcp" && topNodeType != structs.DiscoveryGraphNodeTypeResolver) {
						return fmt.Errorf(
							"service %q has protocol %q, which does not match defined listener protocol %q",
							svcID.String(),
							svcProto,
							expectedProto,
						)
					}
				}
			}
		}
	}

	// Now validate that intentions with L7 permissions reference HTTP services
	for _, e := range checkIntentions {
		// We only have to double check things that try to use permissions
		if e.HasWildcardDestination() || !e.HasAnyPermissions() {
			continue
		}
		sn := e.DestinationServiceName()
		svcID := sn.ToServiceID()

		svcProto := svcProtocols[svcID]
		if !structs.IsProtocolHTTPLike(svcProto) {
			return fmt.Errorf(
				"service %q has protocol %q, which is incompatible with L7 intentions permissions",
				svcID.String(),
				svcProto,
			)
		}
	}

	return nil
}

func validateChainIsPeerExportSafe(
	tx ReadTxn,
	exportedSvc structs.ServiceName,
	overrides map[configentry.KindName]structs.ConfigEntry,
) error {
	_, chainEntries, err := readDiscoveryChainConfigEntriesTxn(tx, nil, exportedSvc.Name, overrides, &exportedSvc.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("error reading discovery chain for %q during config entry validation: %w", exportedSvc, err)
	}

	emptyOrMatchesEntryPartition := func(entry structs.ConfigEntry, found string) bool {
		if found == "" {
			return true
		}
		return acl.EqualPartitions(entry.GetEnterpriseMeta().PartitionOrEmpty(), found)
	}

	for _, e := range chainEntries.Routers {
		for _, route := range e.Routes {
			if route.Destination == nil {
				continue
			}
			if !emptyOrMatchesEntryPartition(e, route.Destination.Partition) {
				return fmt.Errorf("peer exported service %q contains cross-partition route destination", exportedSvc)
			}
		}
	}

	for _, e := range chainEntries.Splitters {
		for _, split := range e.Splits {
			if !emptyOrMatchesEntryPartition(e, split.Partition) {
				return fmt.Errorf("peer exported service %q contains cross-partition split destination", exportedSvc)
			}
		}
	}

	for _, e := range chainEntries.Resolvers {
		if e.Redirect != nil {
			if e.Redirect.Datacenter != "" {
				return fmt.Errorf("peer exported service %q contains cross-datacenter resolver redirect", exportedSvc)
			}
			if !emptyOrMatchesEntryPartition(e, e.Redirect.Partition) {
				return fmt.Errorf("peer exported service %q contains cross-partition resolver redirect", exportedSvc)
			}
		}
		if e.Failover != nil {
			for _, failover := range e.Failover {
				if len(failover.Datacenters) > 0 {
					return fmt.Errorf("peer exported service %q contains cross-datacenter failover", exportedSvc)
				}
			}
		}
	}

	return nil
}

// testCompileDiscoveryChain speculatively compiles a discovery chain with
// pending modifications to see if it would be valid. Also returns the computed
// protocol and topmost discovery chain node.
//
// If provided, the overrides map will service reads of specific config entries
// instead of the state store if the config entry kind name is present in the
// map. A nil in the map implies that the config entry should be tombstoned
// during evaluation and treated as erased.
//
// The override map lets us speculatively compile a discovery chain to see if
// doing so would error, so we can ultimately block config entry writes from
// happening.
func testCompileDiscoveryChain(
	tx ReadTxn,
	chainName string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (string, *structs.DiscoveryGraphNode, map[string]*structs.DiscoveryTarget, error) {
	_, speculativeEntries, err := readDiscoveryChainConfigEntriesTxn(tx, nil, chainName, overrides, entMeta)
	if err != nil {
		return "", nil, nil, err
	}

	// Note we use an arbitrary namespace and datacenter as those would not
	// currently affect the graph compilation in ways that matter here.
	//
	// TODO(rb): we should thread a better value than "dc1" and the throwaway trust domain down here as that is going to sometimes show up in user facing errors
	req := discoverychain.CompileRequest{
		ServiceName:           chainName,
		EvaluateInNamespace:   entMeta.NamespaceOrDefault(),
		EvaluateInPartition:   entMeta.PartitionOrDefault(),
		EvaluateInDatacenter:  "dc1",
		EvaluateInTrustDomain: "b6fc9da3-03d4-4b5a-9134-c045e9b20152.consul",
		Entries:               speculativeEntries,
	}
	chain, err := discoverychain.Compile(req)
	if err != nil {
		return "", nil, nil, err
	}

	return chain.Protocol, chain.Nodes[chain.StartNode], chain.Targets, nil
}

func (s *Store) ServiceDiscoveryChain(
	ws memdb.WatchSet,
	serviceName string,
	entMeta *acl.EnterpriseMeta,
	req discoverychain.CompileRequest,
) (uint64, *structs.CompiledDiscoveryChain, *configentry.DiscoveryChainSet, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return serviceDiscoveryChainTxn(tx, ws, serviceName, entMeta, req)
}

func serviceDiscoveryChainTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	entMeta *acl.EnterpriseMeta,
	req discoverychain.CompileRequest,
) (uint64, *structs.CompiledDiscoveryChain, *configentry.DiscoveryChainSet, error) {

	index, entries, err := readDiscoveryChainConfigEntriesTxn(tx, ws, serviceName, nil, entMeta)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Entries = entries

	_, config, err := caConfigTxn(tx, ws)
	if err != nil {
		return 0, nil, nil, err
	} else if config == nil {
		return 0, nil, nil, errors.New("no cluster ca config setup")
	}

	// Build TrustDomain based on the ClusterID stored.
	signingID := connect.SpiffeIDSigningForCluster(config.ClusterID)
	if signingID == nil {
		// If CA is bootstrapped at all then this should never happen but be
		// defensive.
		return 0, nil, nil, errors.New("no cluster trust domain setup")
	}
	req.EvaluateInTrustDomain = signingID.Host()

	// Then we compile it into something useful.
	chain, err := discoverychain.Compile(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to compile discovery chain: %v", err)
	}

	return index, chain, entries, nil
}

func (s *Store) ReadResolvedServiceConfigEntries(
	ws memdb.WatchSet,
	serviceName string,
	entMeta *acl.EnterpriseMeta,
	upstreamIDs []structs.ServiceID,
	proxyMode structs.ProxyMode,
) (uint64, *configentry.ResolvedServiceConfigSet, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	var res configentry.ResolvedServiceConfigSet

	// The caller will likely calculate this again, but we need to do it here
	// to determine if we are going to traverse into implicit upstream
	// definitions.
	var inferredProxyMode structs.ProxyMode

	index, proxyEntry, err := configEntryTxn(tx, ws, structs.ProxyDefaults, structs.ProxyConfigGlobal, entMeta)
	if err != nil {
		return 0, nil, err
	}
	maxIndex := index

	if proxyEntry != nil {
		var ok bool
		proxyConf, ok := proxyEntry.(*structs.ProxyConfigEntry)
		if !ok {
			return 0, nil, fmt.Errorf("invalid proxy config type %T", proxyEntry)
		}
		res.AddProxyDefaults(proxyConf)

		inferredProxyMode = proxyConf.Mode
	}

	index, serviceEntry, err := configEntryTxn(tx, ws, structs.ServiceDefaults, serviceName, entMeta)
	if err != nil {
		return 0, nil, err
	}

	if index > maxIndex {
		maxIndex = index
	}

	var serviceConf *structs.ServiceConfigEntry
	if serviceEntry != nil {
		var ok bool
		serviceConf, ok = serviceEntry.(*structs.ServiceConfigEntry)
		if !ok {
			return 0, nil, fmt.Errorf("invalid service config type %T", serviceEntry)
		}
		res.AddServiceDefaults(serviceConf)

		if serviceConf.Mode != structs.ProxyModeDefault {
			inferredProxyMode = serviceConf.Mode
		}
	}

	var (
		noUpstreamArgs = len(upstreamIDs) == 0

		// Check the args and the resolved value. If it was exclusively set via a config entry, then proxyMode
		// will never be transparent because the service config request does not use the resolved value.
		tproxy = proxyMode == structs.ProxyModeTransparent || inferredProxyMode == structs.ProxyModeTransparent
	)

	// The upstreams passed as arguments to this endpoint are the upstreams explicitly defined in a proxy registration.
	// If no upstreams were passed, then we should only return the resolved config if the proxy is in transparent mode.
	// Otherwise we would return a resolved upstream config to a proxy with no configured upstreams.
	if noUpstreamArgs && !tproxy {
		return maxIndex, &res, nil
	}

	// First collect all upstreams into a set of seen upstreams.
	// Upstreams can come from:
	// - Explicitly from proxy registrations, and therefore as an argument to this RPC endpoint
	// - Implicitly from centralized upstream config in service-defaults
	seenUpstreams := map[structs.ServiceID]struct{}{}

	for _, sid := range upstreamIDs {
		if _, ok := seenUpstreams[sid]; !ok {
			seenUpstreams[sid] = struct{}{}
		}
	}

	if serviceConf != nil && serviceConf.UpstreamConfig != nil {
		for _, override := range serviceConf.UpstreamConfig.Overrides {
			if override.Name == "" {
				continue // skip this impossible condition
			}
			if override.Peer != "" {
				continue // Peer services do not have service-defaults config entries to fetch.
			}
			sid := override.PeeredServiceName().ServiceName.ToServiceID()
			seenUpstreams[sid] = struct{}{}
		}
	}

	for upstream := range seenUpstreams {
		index, rawEntry, err := configEntryTxn(tx, ws, structs.ServiceDefaults, upstream.ID, &upstream.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}
		if index > maxIndex {
			maxIndex = index
		}

		if rawEntry != nil {
			entry, ok := rawEntry.(*structs.ServiceConfigEntry)
			if !ok {
				return 0, nil, fmt.Errorf("invalid service config type %T", rawEntry)
			}
			res.AddServiceDefaults(entry)
		}
	}

	return maxIndex, &res, nil
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
	entMeta *acl.EnterpriseMeta,
) (uint64, *configentry.DiscoveryChainSet, error) {
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
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (uint64, *configentry.DiscoveryChainSet, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return readDiscoveryChainConfigEntriesTxn(tx, ws, serviceName, overrides, entMeta)
}

func readDiscoveryChainConfigEntriesTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (uint64, *configentry.DiscoveryChainSet, error) {
	res := configentry.NewDiscoveryChainSet()

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

	// At every step we'll need service and proxy defaults.
	todoDefaults[sid] = struct{}{}

	var maxIdx uint64

	// first fetch the router, of which we only collect 1 per chain eval
	idx, router, err := getRouterConfigEntryTxn(tx, ws, serviceName, overrides, entMeta)
	if err != nil {
		return 0, nil, err
	} else if router != nil {
		res.Routers[sid] = router
	}
	if idx > maxIdx {
		maxIdx = idx
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

		idx, splitter, err := getSplitterConfigEntryTxn(tx, ws, splitID.ID, overrides, &splitID.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
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

		idx, resolver, err := getResolverConfigEntryTxn(tx, ws, resolverID.ID, overrides, &resolverID.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
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

		if _, ok := res.ProxyDefaults[svcID.PartitionOrDefault()]; !ok {
			idx, proxy, err := getProxyConfigEntryTxn(tx, ws, structs.ProxyConfigGlobal, overrides, &svcID.EnterpriseMeta)
			if err != nil {
				return 0, nil, err
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			if proxy != nil {
				res.ProxyDefaults[proxy.PartitionOrDefault()] = proxy
			}
		}

		idx, entry, err := getServiceConfigEntryTxn(tx, ws, svcID.ID, overrides, &svcID.EnterpriseMeta)
		if err != nil {
			return 0, nil, err
		}
		if idx > maxIdx {
			maxIdx = idx
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

	return maxIdx, res, nil
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
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getProxyConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	name string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
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
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getServiceConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
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
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getRouterConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
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
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getSplitterConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
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
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getResolverConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
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

// getServiceIntentionsConfigEntryTxn is a convenience method for fetching a
// service-intentions kind of config entry.
//
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getServiceIntentionsConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	name string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (uint64, *structs.ServiceIntentionsConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ServiceIntentions, name, overrides, entMeta)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	ixn, ok := entry.(*structs.ServiceIntentionsConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, ixn, nil
}

// getExportedServicesConfigEntryTxn is a convenience method for fetching a
// exported-services kind of config entry.
//
// If an override KEY is present for the requested config entry, the index
// returned will be 0. Any override VALUE (nil or otherwise) will be returned
// if there is a KEY match.
func getExportedServicesConfigEntryTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (uint64, *structs.ExportedServicesConfigEntry, error) {
	idx, entry, err := configEntryWithOverridesTxn(tx, ws, structs.ExportedServices, entMeta.PartitionOrDefault(), overrides, entMeta)
	if err != nil {
		return 0, nil, err
	} else if entry == nil {
		return idx, nil, nil
	}

	export, ok := entry.(*structs.ExportedServicesConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("invalid service config type %T", entry)
	}
	return idx, export, nil
}

func configEntryWithOverridesTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	kind string,
	name string,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta *acl.EnterpriseMeta,
) (uint64, structs.ConfigEntry, error) {
	if len(overrides) > 0 {
		kn := configentry.NewKindName(kind, name, entMeta)
		kn.Normalize()
		entry, ok := overrides[kn]
		if ok {
			return 0, entry, nil // a nil entry implies it should act like it is erased
		}
	}

	return configEntryTxn(tx, ws, kind, name, entMeta)
}

// protocolForService returns the service graph protocol associated to the
// provided service, checking all relevant config entries.
func protocolForService(
	tx ReadTxn,
	ws memdb.WatchSet,
	svc structs.ServiceName,
) (uint64, string, error) {
	// Get the global proxy defaults (for default protocol)
	maxIdx, proxyConfig, err := getProxyConfigEntryTxn(tx, ws, structs.ProxyConfigGlobal, nil, &svc.EnterpriseMeta)
	if err != nil {
		return 0, "", err
	}

	idx, serviceDefaults, err := getServiceConfigEntryTxn(tx, ws, svc.Name, nil, &svc.EnterpriseMeta)
	if err != nil {
		return 0, "", err
	}
	maxIdx = lib.MaxUint64(maxIdx, idx)

	entries := configentry.NewDiscoveryChainSet()
	if proxyConfig != nil {
		entries.AddEntries(proxyConfig)
	}
	if serviceDefaults != nil {
		entries.AddEntries(serviceDefaults)
	}
	req := discoverychain.CompileRequest{
		ServiceName:          svc.Name,
		EvaluateInNamespace:  svc.NamespaceOrDefault(),
		EvaluateInPartition:  svc.PartitionOrDefault(),
		EvaluateInDatacenter: "dc1",
		// Use a dummy trust domain since that won't affect the protocol here.
		EvaluateInTrustDomain: dummyTrustDomain,
		Entries:               entries,
	}
	chain, err := discoverychain.Compile(req)
	if err != nil {
		return 0, "", err
	}
	return maxIdx, chain.Protocol, nil
}

const dummyTrustDomain = "b6fc9da3-03d4-4b5a-9134-c045e9b20152.consul"

func newConfigEntryQuery(c structs.ConfigEntry) configentry.KindName {
	return configentry.NewKindName(c.GetKind(), c.GetName(), c.GetEnterpriseMeta())
}

// ConfigEntryKindQuery is used to lookup config entries by their kind.
type ConfigEntryKindQuery struct {
	Kind string
	acl.EnterpriseMeta
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q ConfigEntryKindQuery) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q ConfigEntryKindQuery) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

// convertTargetsToTestSpiffeIDs indexes the provided targets by their eventual
// spiffeid values using a dummy trust domain. Returns a map of SpiffeIDs to
// targetID values which can be used for error output.
func convertTargetsToTestSpiffeIDs(targets map[string]*structs.DiscoveryTarget) map[string]string {
	out := make(map[string]string)
	for tid, t := range targets {
		testSpiffeID := connect.SpiffeIDService{
			Host:       dummyTrustDomain,
			Partition:  t.Partition,
			Namespace:  t.Namespace,
			Datacenter: t.Datacenter,
			Service:    t.Service,
		}
		uri := testSpiffeID.URI().String()
		if _, ok := out[uri]; !ok {
			out[uri] = tid
		}
	}
	return out
}
