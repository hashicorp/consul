package state

import (
	"fmt"
	"io"
	"log"

	"github.com/hashicorp/go-memdb"
)

type StateStore struct {
	logger *log.Logger
	db     *memdb.MemDB
}

type IndexEntry struct {
	Key   string
	Value uint64
}

func NewStateStore(logOutput io.Writer) (*StateStore, error) {
	// Create the in-memory DB
	db, err := memdb.NewMemDB(stateStoreSchema())
	if err != nil {
		return nil, fmt.Errorf("Failed setting up state store: %s", err)
	}

	// Create and return the state store
	s := &StateStore{
		logger: log.New(logOutput, "", log.LstdFlags),
		db:     db,
	}
	return s, nil
}
