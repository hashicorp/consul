// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package inmem

import (
	"context"
	"sync"
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
	mu sync.RWMutex // guards db, because Restore.Commit will replace it wholesale.
	db *memdb.MemDB

	pub *stream.EventPublisher

	// eventLock is used to serialize operations that result in the publishing of
	// events (i.e. writes and deletes) to ensure correct ordering when there are
	// concurrent writers.
	//
	// We cannot rely on MemDB's write lock for this, because events must be
	// published *after* the transaction is committed to provide monotonic reads
	// between Watch and Read calls. In other words, if we were to publish an event
	// before the transaction was committed, there would be a small window of time
	// where a watcher (e.g. controller) could try to Read the resource and not get
	// the version they were notified about.
	//
	// Without this lock, it would be possible to publish events out-of-order.
	eventLock sync.Mutex
}

// NewStore creates a Store.
//
// You must call Run before using the store.
func NewStore() (*Store, error) {
	db, err := newDB()
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
	tx := s.txn(false)

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

// WriteCAS performs an atomic Compare-And-Swap (CAS) write of a resource.
//
// For more information, see the storage.Backend documentation.
func (s *Store) WriteCAS(res *pbresource.Resource, vsn string) error {
	s.eventLock.Lock()
	defer s.eventLock.Unlock()

	tx := s.txn(true)
	defer tx.Abort()

	existing, err := tx.First(tableNameResources, indexNameID, res.Id)
	if err != nil {
		return err
	}

	// Callers provide an empty version string on initial resource creation.
	if existing == nil && vsn != "" {
		return storage.ErrCASFailure
	}

	if existing != nil {
		existingRes := existing.(*pbresource.Resource)

		// Uid is immutable.
		if existingRes.Id.Uid != res.Id.Uid {
			return storage.ErrWrongUid
		}

		// Ensure CAS semantics.
		if existingRes.Version != vsn {
			return storage.ErrCASFailure
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

	s.publishEvent(idx, pbresource.WatchEvent_OPERATION_UPSERT, res)

	return nil
}

// DeleteCAS performs an atomic Compare-And-Swap (CAS) deletion of a resource.
//
// For more information, see the storage.Backend documentation.
func (s *Store) DeleteCAS(id *pbresource.ID, vsn string) error {
	s.eventLock.Lock()
	defer s.eventLock.Unlock()

	tx := s.txn(true)
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
		return storage.ErrCASFailure
	}

	if err := tx.Delete(tableNameResources, id); err != nil {
		return err
	}

	idx, err := incrementEventIndex(tx)
	if err != nil {
		return nil
	}
	tx.Commit()

	s.publishEvent(idx, pbresource.WatchEvent_OPERATION_DELETE, res)

	return nil
}

// List resources of the given type, tenancy, and optionally matching the given
// name prefix.
//
// For more information, see the storage.Backend documentation.
func (s *Store) List(typ storage.UnversionedType, ten *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	tx := s.txn(false)
	defer tx.Abort()

	return listTxn(tx, query{typ, ten, namePrefix})
}

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

// ListByOwner returns resources owned by the resource with the given ID.
//
// For more information, see the storage.Backend documentation.
func (s *Store) ListByOwner(id *pbresource.ID) ([]*pbresource.Resource, error) {
	tx := s.txn(false)
	defer tx.Abort()

	iter, err := tx.Get(tableNameResources, indexNameOwner, id)
	if err != nil {
		return nil, err
	}

	var res []*pbresource.Resource
	for v := iter.Next(); v != nil; v = iter.Next() {
		res = append(res, v.(*pbresource.Resource))
	}
	return res, nil
}

func (s *Store) txn(write bool) *memdb.Txn {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db.Txn(write)
}
