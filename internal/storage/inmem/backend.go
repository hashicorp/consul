package inmem

import (
	"context"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// NewBackend returns a purely in-memory storage backend. It's suitable for
// testing and development mode, but should NOT be used in production as it
// has no support for durable persistence, so all of your data will be lost
// when the process restarts or crashes.
//
// You must call Run before using the backend.
func NewBackend() (*Backend, error) {
	store, err := NewStore()
	if err != nil {
		return nil, err
	}
	return &Backend{store}, nil
}

// Backend is a purely in-memory storage backend implementation.
type Backend struct{ store *Store }

// Run until the given context is canceled. This method blocks, so should be
// called in a goroutine.
func (b *Backend) Run(ctx context.Context) { b.store.Run(ctx) }

// Read implements the storage.Backend interface.
func (b *Backend) Read(_ context.Context, id *pbresource.ID) (*pbresource.Resource, error) {
	return b.store.Read(id)
}

// ReadConsistent implements the storage.Backend interface.
func (b *Backend) ReadConsistent(_ context.Context, id *pbresource.ID) (*pbresource.Resource, error) {
	return b.store.Read(id)
}

// WriteCAS implements the storage.Backend interface.
func (b *Backend) WriteCAS(_ context.Context, res *pbresource.Resource, version string) (*pbresource.Resource, error) {
	if err := b.store.WriteCAS(res, version); err != nil {
		return nil, err
	}
	return res, nil
}

// DeleteCAS implements the storage.Backend interface.
func (b *Backend) DeleteCAS(_ context.Context, id *pbresource.ID, version string) error {
	return b.store.DeleteCAS(id, version)
}

// List implements the storage.Backend interface.
func (b *Backend) List(_ context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) ([]*pbresource.Resource, error) {
	return b.store.List(resType, tenancy, namePrefix)
}

// WatchList implements the storage.Backend interface.
func (b *Backend) WatchList(_ context.Context, resType storage.UnversionedType, tenancy *pbresource.Tenancy, namePrefix string) (storage.Watch, error) {
	return b.store.WatchList(resType, tenancy, namePrefix)
}

// OwnerReferences implements the storage.Backend interface.
func (b *Backend) OwnerReferences(_ context.Context, id *pbresource.ID) ([]*pbresource.ID, error) {
	return b.store.OwnerReferences(id)
}
