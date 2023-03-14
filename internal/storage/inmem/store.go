package inmem

import (
	"context"
	"time"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Store implements an in-memory resource database using go-memdb.
//
// It can be used as a storage backend directly via the Backend type in this
// package, but also handles reads in our Raft backend, and can be used as a
// local cache when storing data in external systems (e.g. RDBMS, K/V stores).
type Store struct {
	db  *memdb.MemDB
	pub *stream.EventPublisher
}

// NewStore creates a Store.
//
// You must call Run before using the store.
func NewStore() (*Store, error) {
	db, err := memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			tableNameMetadata: {
				Name: tableNameMetadata,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:         indexNameID,
						AllowMissing: false,
						Unique:       true,
						Indexer:      &memdb.StringFieldIndex{Field: "Key"},
					},
				},
			},
			tableNameResources: {
				Name: tableNameResources,
				Indexes: map[string]*memdb.IndexSchema{
					indexNameID: {
						Name:         indexNameID,
						AllowMissing: false,
						Unique:       true,
						Indexer:      idIndexer{},
					},
					indexNameOwner: {
						Name:         indexNameOwner,
						AllowMissing: true,
						Unique:       false,
						Indexer:      ownerIndexer{},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	s := &Store{
		db:  db,
		pub: stream.NewEventPublisher(10 * time.Second),
	}
	s.pub.RegisterHandler(eventTopic, s.watchSnapshot, false)

	return s, nil
}

// Run until the given context is canceled. This method blocks, so should be
// called in a goroutine.
func (s *Store) Run(ctx context.Context) { s.pub.Run(ctx) }

// Read a resource using its ID.
//
// For more information, see the storage.Backend documentation.
func (s *Store) Read(id *pbresource.ID) (*pbresource.Resource, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	val, err := tx.First(tableNameResources, indexNameID, id)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, storage.ErrNotFound
	}

	res := val.(*pbresource.Resource)

	// Observe the Uid if it was given.
	if id.Uid != "" && res.Id.Uid != id.Uid {
		return nil, storage.ErrNotFound
	}

	// Let the caller know they need to upgrade/downgrade the schema version.
	if id.Type.GroupVersion != res.Id.Type.GroupVersion {
		return nil, storage.GroupVersionMismatchError{
			RequestedType: id.Type,
			Stored:        res,
		}
	}

	return res, nil
}

// WriteCAS performs an atomic Check-And-Set (CAS) write of a resource.
//
// For more information, see the storage.Backend documentation.
func (s *Store) WriteCAS(res *pbresource.Resource, vsn string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	existing, err := tx.First(tableNameResources, indexNameID, res.Id)
	if err != nil {
		return err
	}

	if existing == nil && vsn != "" {
		return storage.ErrConflict
	}

	if existing != nil {
		existingRes := existing.(*pbresource.Resource)

		// Ensure CAS semantics.
		if existingRes.Version != vsn {
			return storage.ErrConflict
		}

		// Uid is immutable.
		if existingRes.Id.Uid != res.Id.Uid {
			return storage.ErrConflict
		}
	}

	if err := tx.Insert(tableNameResources, res); err != nil {
		return err
	}

	idx, err := incrementEventIndex(tx)
	if err != nil {
		return nil
	}
	tx.Commit()

	s.publishEvent(idx, &pbresource.WatchEvent{
		Operation: pbresource.WatchEvent_OPERATION_UPSERT,
		Resource:  res,
	})

	return nil
}

// DeleteCAS performs an atomic Check-And-Set (CAS) deletion of a resource.
//
// For more information, see the storage.Backend documentation.
func (s *Store) DeleteCAS(id *pbresource.ID, vsn string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	existing, err := tx.First(tableNameResources, indexNameID, id)
	if err != nil {
		return err
	}

	// Deleting an already deleted resource is a no-op.
	if existing == nil {
		return nil
	}

	res := existing.(*pbresource.Resource)

	// Deleting a resource using a previous Uid is a no-op.
	if id.Uid != res.Id.Uid {
		return nil
	}

	// Ensure CAS semantics.
	if vsn != res.Version {
		return storage.ErrConflict
	}

	if err := tx.Delete(tableNameResources, id); err != nil {
		return err
	}

	idx, err := incrementEventIndex(tx)
	if err != nil {
		return nil
	}
	tx.Commit()

	s.publishEvent(idx, &pbresource.WatchEvent{
		Operation: pbresource.WatchEvent_OPERATION_DELETE,
		Resource:  res,
	})

	return nil
}

// List resources of the given type, tenancy, and optionally matching the given
// name prefix.
//
// For more information, see the storage.Backend documentation.
func (s *Store) List(typ storage.UnversionedType, ten *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	return listTxn(tx, query{typ, ten, namePrefix})
}

// WatchList watches resources of the given type, tenancy, and optionally
// matching the given name prefix.
//
// For more information, see the storage.Backend documentation.
func (s *Store) WatchList(typ storage.UnversionedType, ten *pbresource.Tenancy, namePrefix string) (*Watch, error) {
	// If the user specifies a wildcard, we subscribe to events for resources in
	// all partitions, peers, and namespaces, and manually filter out irrelevant
	// stuff (in Watch.Next).
	//
	// If the user gave exact tenancy values, we can subscribe to events for the
	// relevant resources only, which is far more efficient.
	var sub stream.Subject
	if ten.Partition == storage.Wildcard ||
		ten.PeerName == storage.Wildcard ||
		ten.Namespace == storage.Wildcard {
		sub = wildcardSubject{typ}
	} else {
		sub = tenancySubject{typ, ten}
	}

	ss, err := s.pub.Subscribe(&stream.SubscribeRequest{
		Topic:   eventTopic,
		Subject: sub,
	})
	if err != nil {
		return nil, err
	}

	return &Watch{
		sub: ss,
		query: query{
			resourceType: typ,
			tenancy:      ten,
			namePrefix:   namePrefix,
		},
	}, nil
}

// OwnerReferences returns the IDs of resources owned by the resource with the
// given ID.
//
// For more information, see the storage.Backend documentation.
func (s *Store) OwnerReferences(id *pbresource.ID) ([]*pbresource.ID, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	iter, err := tx.Get(tableNameResources, indexNameOwner, id)
	if err != nil {
		return nil, err
	}

	var refs []*pbresource.ID
	for v := iter.Next(); v != nil; v = iter.Next() {
		refs = append(refs, v.(*pbresource.Resource).Id)
	}
	return refs, nil
}

const (
	tableNameMetadata  = "metadata"
	tableNameResources = "resources"

	indexNameID    = "id"
	indexNameOwner = "owner"

	metaKeyEventIndex = "index"
)

func listTxn(tx *memdb.Txn, q query) ([]*pbresource.Resource, error) {
	iter, err := tx.Get(tableNameResources, indexNameID+"_prefix", q)
	if err != nil {
		return nil, err
	}

	list := make([]*pbresource.Resource, 0)
	for v := iter.Next(); v != nil; v = iter.Next() {
		res := v.(*pbresource.Resource)

		if q.matches(res) {
			list = append(list, res)
		}
	}
	return list, nil
}

type meta struct {
	Key   string
	Value any
}

func incrementEventIndex(tx *memdb.Txn) (uint64, error) {
	idx, err := currentEventIndex(tx)
	if err != nil {
		return 0, err
	}

	idx++
	if err := tx.Insert(tableNameMetadata, meta{Key: metaKeyEventIndex, Value: idx}); err != nil {
		return 0, nil
	}
	return idx, nil
}

func currentEventIndex(tx *memdb.Txn) (uint64, error) {
	v, err := tx.First(tableNameMetadata, indexNameID, metaKeyEventIndex)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return v.(meta).Value.(uint64), nil
}
