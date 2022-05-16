package state

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

const (
	tablePeering             = "peering"
	tablePeeringTrustBundles = "peering-trust-bundles"
)

func peeringTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tablePeering,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle{
					readIndex:  readIndex(indexFromUUIDString),
					writeIndex: writeIndex(indexIDFromPeering),
				},
			},
			indexName: {
				Name:         indexName,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix{
					readIndex:   indexPeeringFromQuery,
					writeIndex:  indexFromPeering,
					prefixIndex: prefixIndexFromQueryNoNamespace,
				},
			},
		},
	}
}

func peeringTrustBundlesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tablePeeringTrustBundles,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle{
					readIndex:  indexPeeringFromQuery, // same as peering table since we'll use the query.Value
					writeIndex: indexFromPeeringTrustBundle,
				},
			},
		},
	}
}

func indexIDFromPeering(raw interface{}) ([]byte, error) {
	p, ok := raw.(*pbpeering.Peering)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for pbpeering.Peering index", raw)
	}

	if p.ID == "" {
		return nil, errMissingValueForIndex
	}

	uuid, err := uuidStringToBytes(p.ID)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func (s *Store) PeeringReadByID(ws memdb.WatchSet, id string) (uint64, *pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	peering, err := peeringReadByIDTxn(tx, ws, id)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read peering by id: %w", err)
	}
	if peering == nil {
		// Return the tables index so caller can watch it for changes if the peering doesn't exist
		return maxIndexWatchTxn(tx, ws, tablePeering), nil, nil
	}

	return peering.ModifyIndex, peering, nil
}

func peeringReadByIDTxn(tx ReadTxn, ws memdb.WatchSet, id string) (*pbpeering.Peering, error) {
	watchCh, peeringRaw, err := tx.FirstWatch(tablePeering, indexID, id)
	if err != nil {
		return nil, fmt.Errorf("failed peering lookup: %w", err)
	}
	ws.Add(watchCh)

	peering, ok := peeringRaw.(*pbpeering.Peering)
	if peeringRaw != nil && !ok {
		return nil, fmt.Errorf("invalid type %T", peering)
	}
	return peering, nil
}

func (s *Store) PeeringRead(ws memdb.WatchSet, q Query) (uint64, *pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return peeringReadTxn(tx, ws, q)
}

func peeringReadTxn(tx ReadTxn, ws memdb.WatchSet, q Query) (uint64, *pbpeering.Peering, error) {
	watchCh, peeringRaw, err := tx.FirstWatch(tablePeering, indexName, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering lookup: %w", err)
	}

	peering, ok := peeringRaw.(*pbpeering.Peering)
	if peeringRaw != nil && !ok {
		return 0, nil, fmt.Errorf("invalid type %T", peering)
	}
	ws.Add(watchCh)

	if peering == nil {
		// Return the tables index so caller can watch it for changes if the peering doesn't exist
		return maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeering, q.PartitionOrDefault())), nil, nil
	}

	return peering.ModifyIndex, peering, nil
}

func (s *Store) PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	var (
		iter memdb.ResultIterator
		err  error
		idx  uint64
	)
	if entMeta.PartitionOrDefault() == structs.WildcardSpecifier {
		iter, err = tx.Get(tablePeering, indexID)
		idx = maxIndexWatchTxn(tx, ws, tablePeering)
	} else {
		iter, err = tx.Get(tablePeering, indexName+"_prefix", entMeta)
		idx = maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeering, entMeta.PartitionOrDefault()))
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering lookup: %v", err)
	}

	var result []*pbpeering.Peering
	for entry := iter.Next(); entry != nil; entry = iter.Next() {
		result = append(result, entry.(*pbpeering.Peering))
	}

	return idx, result, nil
}

func generatePeeringUUID(tx ReadTxn) (string, error) {
	for {
		uuid, err := uuid.GenerateUUID()
		if err != nil {
			return "", fmt.Errorf("failed to generate UUID: %w", err)
		}
		existing, err := peeringReadByIDTxn(tx, nil, uuid)
		if err != nil {
			return "", fmt.Errorf("failed to read peering: %w", err)
		}
		if existing == nil {
			return uuid, nil
		}
	}
}

func (s *Store) PeeringWrite(idx uint64, p *pbpeering.Peering) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	q := Query{
		Value:          p.Name,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(p.Partition),
	}
	existingRaw, err := tx.First(tablePeering, indexName, q)
	if err != nil {
		return fmt.Errorf("failed peering lookup: %w", err)
	}

	existing, ok := existingRaw.(*pbpeering.Peering)
	if existingRaw != nil && !ok {
		return fmt.Errorf("invalid type %T", existingRaw)
	}

	if existing != nil {
		p.CreateIndex = existing.CreateIndex
		p.ID = existing.ID

	} else {
		// TODO(peering): consider keeping PeeringState enum elsewhere?
		p.State = pbpeering.PeeringState_INITIAL
		p.CreateIndex = idx

		p.ID, err = generatePeeringUUID(tx)
		if err != nil {
			return fmt.Errorf("failed to generate peering id: %w", err)
		}
	}
	p.ModifyIndex = idx

	if err := tx.Insert(tablePeering, p); err != nil {
		return fmt.Errorf("failed inserting peering: %w", err)
	}

	if err := updatePeeringTableIndexes(tx, idx, p.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

// TODO(peering): replace with deferred deletion since this operation
// should involve cleanup of data associated with the peering.
func (s *Store) PeeringDelete(idx uint64, q Query) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := tx.First(tablePeering, indexName, q)
	if err != nil {
		return fmt.Errorf("failed peering lookup: %v", err)
	}

	if existing == nil {
		return nil
	}

	if err := tx.Delete(tablePeering, existing); err != nil {
		return fmt.Errorf("failed deleting peering: %v", err)
	}

	if err := updatePeeringTableIndexes(tx, idx, q.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PeeringTerminateByID(idx uint64, id string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := peeringReadByIDTxn(tx, nil, id)
	if err != nil {
		return fmt.Errorf("failed to read peering %q: %w", id, err)
	}
	if existing == nil {
		return nil
	}

	c := proto.Clone(existing)
	clone, ok := c.(*pbpeering.Peering)
	if !ok {
		return fmt.Errorf("invalid type %T, expected *pbpeering.Peering", existing)
	}

	clone.State = pbpeering.PeeringState_TERMINATED
	clone.ModifyIndex = idx

	if err := tx.Insert(tablePeering, clone); err != nil {
		return fmt.Errorf("failed inserting peering: %w", err)
	}

	if err := updatePeeringTableIndexes(tx, idx, clone.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

// ExportedServicesForPeer returns the list of typical and proxy services exported to a peer.
// TODO(peering): What to do about terminating gateways? Sometimes terminating gateways are the appropriate destination
//                to dial for an upstream mesh service. However, that information is handled by observing the terminating gateway's
//                config entry, which we wouldn't want to replicate. How would client peers know to route through terminating gateways
//                when they're not dialing through a remote mesh gateway?
func (s *Store) ExportedServicesForPeer(ws memdb.WatchSet, peerID string) (uint64, []structs.ServiceName, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	peering, err := peeringReadByIDTxn(tx, ws, peerID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read peering: %w", err)
	}
	if peering == nil {
		return 0, nil, nil
	}

	maxIdx := peering.ModifyIndex

	entMeta := structs.NodeEnterpriseMetaInPartition(peering.Partition)
	idx, raw, err := configEntryTxn(tx, ws, structs.ExportedServices, entMeta.PartitionOrDefault(), entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to fetch exported-services config entry: %w", err)
	}
	if idx > maxIdx {
		maxIdx = idx
	}
	if raw == nil {
		return maxIdx, nil, nil
	}
	conf, ok := raw.(*structs.ExportedServicesConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("expected type *structs.ExportedServicesConfigEntry, got %T", raw)
	}

	set := make(map[structs.ServiceName]struct{})

	for _, svc := range conf.Services {
		svcMeta := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), svc.Namespace)

		sawPeer := false
		for _, consumer := range svc.Consumers {
			name := structs.NewServiceName(svc.Name, &svcMeta)

			if _, ok := set[name]; ok {
				// Service was covered by a wildcard that was already accounted for
				continue
			}
			if consumer.PeerName != peering.Name {
				continue
			}
			sawPeer = true

			if svc.Name != structs.WildcardSpecifier {
				set[name] = struct{}{}
			}
		}

		// If the target peer is a consumer, and all services in the namespace are exported, query those service names.
		if sawPeer && svc.Name == structs.WildcardSpecifier {
			var typicalServices []*KindServiceName
			idx, typicalServices, err = serviceNamesOfKindTxn(tx, ws, structs.ServiceKindTypical, svcMeta)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to get service names: %w", err)
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			for _, s := range typicalServices {
				set[s.Service] = struct{}{}
			}

			var proxyServices []*KindServiceName
			idx, proxyServices, err = serviceNamesOfKindTxn(tx, ws, structs.ServiceKindConnectProxy, svcMeta)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to get service names: %w", err)
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			for _, s := range proxyServices {
				set[s.Service] = struct{}{}
			}
		}
	}

	var resp []structs.ServiceName
	for svc := range set {
		resp = append(resp, svc)
	}
	return maxIdx, resp, nil
}

// PeeringsForService returns the list of peerings that are associated with the service name provided in the query.
// This is used to configure connect proxies for a given service. The result is generated by querying for exported
// service config entries and filtering for those that match the given service.
// TODO(peering): this implementation does all of the work on read to materialize this list of peerings, we should explore
// writing to a separate index that has service peerings prepared ahead of time should this become a performance bottleneck.
func (s *Store) PeeringsForService(ws memdb.WatchSet, serviceName string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	// short-circuit if the service does not exist in the context of the query -- this prevents "leaking" services
	// when there are wildcard rules in place.
	if svcIdx, svcExists, err := serviceExists(tx, ws, serviceName, &entMeta, ""); err != nil {
		return 0, nil, fmt.Errorf("failed to check if service exists: %w", err)
	} else if !svcExists {
		// if the service does not exist, return the max index for the services table so caller can watch for changes
		return svcIdx, nil, nil
	}
	// config entries must be defined in the default namespace, so we only need the partition here
	meta := structs.DefaultEnterpriseMetaInPartition(entMeta.PartitionOrDefault())
	// return the idx of the config entry that was last modified so caller can watch for changes
	idx, peeredServices, err := readPeeredServicesFromConfigEntriesTxn(tx, ws, serviceName, meta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read peered services for service name: %w", err)
	}

	var peerings []*pbpeering.Peering

	// lookup the peering for each matching peered service
	for _, peeredService := range peeredServices {
		readQuery := Query{
			Value:          peeredService.PeerName,
			EnterpriseMeta: peeredService.Name.EnterpriseMeta,
		}
		_, peering, err := peeringReadTxn(tx, ws, readQuery)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read peering: %w", err)
		}
		if peering == nil {
			continue
		}
		peerings = append(peerings, peering)
	}
	// see note above about idx
	return idx, peerings, nil
}

// PeeringTrustBundleRead returns the peering trust bundle for the peer name given as the query value.
func (s *Store) PeeringTrustBundleRead(ws memdb.WatchSet, q Query) (uint64, *pbpeering.PeeringTrustBundle, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	watchCh, ptbRaw, err := tx.FirstWatch(tablePeeringTrustBundles, indexID, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering trust bundle lookup: %w", err)
	}

	ptb, ok := ptbRaw.(*pbpeering.PeeringTrustBundle)
	if ptbRaw != nil && !ok {
		return 0, nil, fmt.Errorf("invalid type %T", ptb)
	}
	ws.Add(watchCh)

	if ptb == nil {
		// Return the tables index so caller can watch it for changes if the trust bundle doesn't exist
		return maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeeringTrustBundles, q.PartitionOrDefault())), nil, nil
	}
	return ptb.ModifyIndex, ptb, nil
}

// PeeringTrustBundleWrite writes ptb to the state store. If there is an existing trust bundle with the given peer name,
// it will be overwritten.
func (s *Store) PeeringTrustBundleWrite(idx uint64, ptb *pbpeering.PeeringTrustBundle) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	q := Query{
		Value:          ptb.PeerName,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(ptb.Partition),
	}
	existingRaw, err := tx.First(tablePeeringTrustBundles, indexID, q)
	if err != nil {
		return fmt.Errorf("failed peering trust bundle lookup: %w", err)
	}

	existing, ok := existingRaw.(*pbpeering.PeeringTrustBundle)
	if existingRaw != nil && !ok {
		return fmt.Errorf("invalid type %T", existingRaw)
	}

	if existing != nil {
		ptb.CreateIndex = existing.CreateIndex

	} else {
		ptb.CreateIndex = idx
	}

	ptb.ModifyIndex = idx

	if err := tx.Insert(tablePeeringTrustBundles, ptb); err != nil {
		return fmt.Errorf("failed inserting peering trust bundle: %w", err)
	}

	if err := updatePeeringTrustBundlesTableIndexes(tx, idx, ptb.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PeeringTrustBundleDelete(idx uint64, q Query) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := tx.First(tablePeeringTrustBundles, indexID, q)
	if err != nil {
		return fmt.Errorf("failed peering trust bundle lookup: %v", err)
	}

	if existing == nil {
		return nil
	}

	if err := tx.Delete(tablePeeringTrustBundles, existing); err != nil {
		return fmt.Errorf("failed deleting peering trust bundle: %v", err)
	}

	if err := updatePeeringTrustBundlesTableIndexes(tx, idx, q.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Snapshot) Peerings() (memdb.ResultIterator, error) {
	return s.tx.Get(tablePeering, indexName)
}

func (s *Snapshot) PeeringTrustBundles() (memdb.ResultIterator, error) {
	return s.tx.Get(tablePeeringTrustBundles, indexID)
}

func (r *Restore) Peering(p *pbpeering.Peering) error {
	if err := r.tx.Insert(tablePeering, p); err != nil {
		return fmt.Errorf("failed restoring peering: %w", err)
	}

	if err := updatePeeringTableIndexes(r.tx, p.ModifyIndex, p.PartitionOrDefault()); err != nil {
		return err
	}

	return nil
}

func (r *Restore) PeeringTrustBundle(ptb *pbpeering.PeeringTrustBundle) error {
	if err := r.tx.Insert(tablePeeringTrustBundles, ptb); err != nil {
		return fmt.Errorf("failed restoring peering trust bundle: %w", err)
	}
	if err := updatePeeringTrustBundlesTableIndexes(r.tx, ptb.ModifyIndex, ptb.PartitionOrDefault()); err != nil {
		return err
	}
	return nil
}

// readPeeredServicesFromConfigEntriesTxn queries exported-service config entries to return peers for serviceName
// in the form of a []structs.PeeredService.
func readPeeredServicesFromConfigEntriesTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	entMeta *acl.EnterpriseMeta,
) (uint64, []structs.PeeredService, error) {
	var results []structs.PeeredService

	// Get all exported-service config entries for that have exports for serviceName. This assumes the result
	// has exported services filtered to only those matching serviceName so no futher filtering is needed.
	idx, exportedServicesEntries, err := getExportedServiceConfigEntriesTxn(tx, ws, serviceName, entMeta)
	if err != nil {
		return 0, nil, err
	}

	// dedupe results by peer name
	resultSet := make(map[string]struct{})
	// filter entries to only those that have a peer consumer defined
	for _, entry := range exportedServicesEntries {
		for _, service := range entry.Services {
			// entries must have consumers
			if service.Consumers == nil || len(service.Consumers) == 0 {
				continue
			}
			for _, consumer := range service.Consumers {
				// and consumers must have a peer
				if consumer.PeerName == "" {
					continue
				}
				// if we get here, we have a peer consumer, but we should dedupe peer names, so skip if it's already in the set
				if _, ok := resultSet[consumer.PeerName]; ok {
					continue
				}

				// if we got here, we can add to the result set
				resultSet[consumer.PeerName] = struct{}{}
				result := structs.PeeredService{
					Name:     structs.NewServiceName(serviceName, entry.GetEnterpriseMeta()),
					PeerName: consumer.PeerName,
				}
				results = append(results, result)
			}
		}
	}
	return idx, results, nil
}
