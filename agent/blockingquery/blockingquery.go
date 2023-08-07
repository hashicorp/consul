// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package blockingquery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/lib"
)

// Sentinel errors that must be used with blockingQuery
var (
	ErrNotFound   = fmt.Errorf("no data found for query")
	ErrNotChanged = fmt.Errorf("data did not change for query")
)

// QueryFn is used to perform a query operation. See Server.blockingQuery for
// the requirements of this function.
type QueryFn func(memdb.WatchSet, *state.Store) error

// RequestOptions are options used by Server.blockingQuery to modify the
// behaviour of the query operation, or to populate response metadata.
type RequestOptions interface {
	GetToken() string
	GetMinQueryIndex() uint64
	GetMaxQueryTime() (time.Duration, error)
	GetRequireConsistent() bool
}

// ResponseMeta is an interface used to populate the response struct
// with metadata about the query and the state of the server.
type ResponseMeta interface {
	SetLastContact(time.Duration)
	SetKnownLeader(bool)
	GetIndex() uint64
	SetIndex(uint64)
	SetResultsFilteredByACLs(bool)
}

// FSMServer is interface into the stateful components of a Consul server, such
// as memdb or raft leadership.
type FSMServer interface {
	ConsistentRead() error
	DecrementBlockingQueries() uint64
	GetShutdownChannel() chan struct{}
	GetState() *state.Store
	IncrementBlockingQueries() uint64
	RPCQueryTimeout(time.Duration) time.Duration
	SetQueryMeta(ResponseMeta, string)
}

// Query performs a blocking query if opts.GetMinQueryIndex is
// greater than 0, otherwise performs a non-blocking query. Blocking queries will
// block until responseMeta.Index is greater than opts.GetMinQueryIndex,
// or opts.GetMaxQueryTime is reached. Non-blocking queries return immediately
// after performing the query.
//
// If opts.GetRequireConsistent is true, blockingQuery will first verify it is
// still the cluster leader before performing the query.
//
// The query function is expected to be a closure that has access to responseMeta
// so that it can set the Index. The actual result of the query is opaque to blockingQuery.
//
// The query function can return ErrNotFound, which is a sentinel error. Returning
// ErrNotFound indicates that the query found no results, which allows
// blockingQuery to keep blocking until the query returns a non-nil error.
// The query function must take care to set the actual result of the query to
// nil in these cases, otherwise when blockingQuery times out it may return
// a previous result. ErrNotFound will never be returned to the caller, it is
// converted to nil before returning.
//
// The query function can return ErrNotChanged, which is a sentinel error. This
// can only be returned on calls AFTER the first call, as it would not be
// possible to detect the absence of a change on the first call. Returning
// ErrNotChanged indicates that the query results are identical to the prior
// results which allows blockingQuery to keep blocking until the query returns
// a real changed result.
//
// The query function must take care to ensure the actual result of the query
// is either left unmodified or explicitly left in a good state before
// returning, otherwise when blockingQuery times out it may return an
// incomplete or unexpected result. ErrNotChanged will never be returned to the
// caller, it is converted to nil before returning.
//
// If query function returns any other error, the error is returned to the caller
// immediately.
//
// The query function must follow these rules:
//
//  1. to access data it must use the passed in state.Store.
//  2. it must set the responseMeta.Index to an index greater than
//     opts.GetMinQueryIndex if the results return by the query have changed.
//  3. any channels added to the memdb.WatchSet must unblock when the results
//     returned by the query have changed.
//
// To ensure optimal performance of the query, the query function should make a
// best-effort attempt to follow these guidelines:
//
//  1. only set responseMeta.Index to an index greater than
//     opts.GetMinQueryIndex when the results returned by the query have changed.
//  2. any channels added to the memdb.WatchSet should only unblock when the
//     results returned by the query have changed.
func Query(
	fsmServer FSMServer,
	requestOpts RequestOptions,
	responseMeta ResponseMeta,
	query QueryFn,
) error {
	var ctx context.Context = &lib.StopChannelContext{StopCh: fsmServer.GetShutdownChannel()}

	metrics.IncrCounter([]string{"rpc", "query"}, 1)

	minQueryIndex := requestOpts.GetMinQueryIndex()
	// Perform a non-blocking query
	if minQueryIndex == 0 {
		if requestOpts.GetRequireConsistent() {
			if err := fsmServer.ConsistentRead(); err != nil {
				return err
			}
		}

		var ws memdb.WatchSet
		err := query(ws, fsmServer.GetState())
		fsmServer.SetQueryMeta(responseMeta, requestOpts.GetToken())
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotChanged) {
			return nil
		}
		return err
	}

	maxQueryTimeout, err := requestOpts.GetMaxQueryTime()
	if err != nil {
		return err
	}
	timeout := fsmServer.RPCQueryTimeout(maxQueryTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	count := fsmServer.IncrementBlockingQueries()
	metrics.SetGauge([]string{"rpc", "queries_blocking"}, float32(count))
	// decrement the count when the function returns.
	defer fsmServer.DecrementBlockingQueries()

	var (
		notFound bool
		ranOnce  bool
	)

	for {
		if requestOpts.GetRequireConsistent() {
			if err := fsmServer.ConsistentRead(); err != nil {
				return err
			}
		}

		// Operate on a consistent set of state. This makes sure that the
		// abandon channel goes with the state that the caller is using to
		// build watches.
		store := fsmServer.GetState()

		ws := memdb.NewWatchSet()
		// This channel will be closed if a snapshot is restored and the
		// whole state store is abandoned.
		ws.Add(store.AbandonCh())

		err := query(ws, store)
		fsmServer.SetQueryMeta(responseMeta, requestOpts.GetToken())

		switch {
		case errors.Is(err, ErrNotFound):
			if notFound {
				// query result has not changed
				minQueryIndex = responseMeta.GetIndex()
			}
			notFound = true
		case errors.Is(err, ErrNotChanged):
			if ranOnce {
				// query result has not changed
				minQueryIndex = responseMeta.GetIndex()
			}
		case err != nil:
			return err
		}
		ranOnce = true

		if responseMeta.GetIndex() > minQueryIndex {
			return nil
		}

		// block until something changes, or the timeout
		if err := ws.WatchCtx(ctx); err != nil {
			// exit if we've reached the timeout, or other cancellation
			return nil
		}

		// exit if the state store has been abandoned
		select {
		case <-store.AbandonCh():
			return nil
		default:
		}
	}
}
