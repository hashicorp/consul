package watch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-memdb"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/consul/lib/retry"
)

var (
	ErrorNotFound   = errors.New("no data found for query")
	ErrorNotChanged = errors.New("data did not change for query")

	errNilContext  = errors.New("cannot call ServerLocalNotify with a nil context")
	errNilGetStore = errors.New("cannot call ServerLocalNotify without a callback to get a StateStore")
	errNilQuery    = errors.New("cannot call ServerLocalNotify without a callback to perform the query")
	errNilNotify   = errors.New("cannot call ServerLocalNotify without a callback to send notifications")
)

//go:generate mockery --name StateStore --inpackage --filename mock_StateStore_test.go
type StateStore interface {
	AbandonCh() <-chan struct{}
}

const (
	defaultWaiterMinFailures uint = 1
	defaultWaiterMinWait          = time.Second
	defaultWaiterMaxWait          = 60 * time.Second
	defaultWaiterFactor           = 2 * time.Second
)

var (
	defaultWaiterJitter = retry.NewJitter(100)
)

func defaultWaiter() *retry.Waiter {
	return &retry.Waiter{
		MinFailures: defaultWaiterMinFailures,
		MinWait:     defaultWaiterMinWait,
		MaxWait:     defaultWaiterMaxWait,
		Jitter:      defaultWaiterJitter,
		Factor:      defaultWaiterFactor,
	}
}

// noopDone can be passed to serverLocalNotifyWithWaiter
func noopDone() {}

// ServerLocalBlockingQuery performs a blocking query similar to the pre-existing blockingQuery
// method on the agent/consul.Server type. There are a few key differences.
//
//  1. This function makes use of Go 1.18 generics. The function is parameterized with two
//     types. The first is the ResultType which can be anything. Having this be parameterized
//     instead of using interface{} allows us to simplify the call sites so that no type
//     coercion from interface{} to the real type is necessary. The second parameterized type
//     is something that VERY loosely resembles a agent/consul/state.Store type. The StateStore
//     interface in this package has a single method to get the stores abandon channel so we
//     know when a snapshot restore is occurring and can act accordingly. We could have not
//     parameterized this type and used a real *state.Store instead but then we would have
//     concrete dependencies on the state package and it would make it a little harder to
//     test this function.
//
//     We could have also avoided the need to use a ResultType parameter by taking the route
//     the original blockingQuery method did and to just assume all callers close around
//     a pointer to their results and can modify it as necessary. That way of doing things
//     feels a little gross so I have taken this one a different direction. The old way
//     also gets especially gross with how we have to push concerns of spurious wakeup
//     suppression down into every call site.
//
//  2. This method has no internal timeout and can potentially run forever until a state
//     change is observed. If there is a desire to have a timeout, that should be built into
//     the context.Context passed as the first argument.
//
//  3. This method bakes in some newer functionality around hashing of results to prevent sending
//     back data when nothing has actually changed. With the old blockingQuery method this has to
//     be done within the closure passed to the method which means the same bit of code is duplicated
//     in many places. As this functionality isn't necessary in many scenarios whether to opt-in to
//     that behavior is a argument to this function.
//
// Similar to the older method:
//
// 1. Errors returned from the query will be propagated back to the caller.
//
// The query function must follow these rules:
//
//  1. To access data it must use the passed in StoreType (which will be a state.Store when
//     everything gets stiched together outside of unit tests).
//  2. It must return an index greater than the minIndex if the results returned by the query
//     have changed.
//  3. Any channels added to the memdb.WatchSet must unblock when the results
//     returned by the query have changed.
//
// To ensure optimal performance of the query, the query function should make a
// best-effort attempt to follow these guidelines:
//
//  1. Only return an index greater than the minIndex.
//  2. Any channels added to the memdb.WatchSet should only unblock when the
//     results returned by the query have changed. This might be difficult
//     to do when blocking on non-existent data.
func ServerLocalBlockingQuery[ResultType any, StoreType StateStore](
	ctx context.Context,
	getStore func() StoreType,
	minIndex uint64,
	suppressSpuriousWakeup bool,
	query func(memdb.WatchSet, StoreType) (uint64, ResultType, error),
) (uint64, ResultType, error) {
	var (
		notFound  bool
		ranOnce   bool
		priorHash uint64
	)

	var zeroResult ResultType
	if getStore == nil {
		return 0, zeroResult, fmt.Errorf("no getStore function was provided to ServerLocalBlockingQuery")
	}
	if query == nil {
		return 0, zeroResult, fmt.Errorf("no query function was provided to ServerLocalBlockingQuery")
	}

	for {
		state := getStore()

		ws := memdb.NewWatchSet()

		// Adding the AbandonCh to the WatchSet allows us to detect when
		// a snapshot restore happens that would otherwise not modify anything
		// within the individual state store. If we didn't do this then we
		// could end up blocking indefinitely.
		ws.Add(state.AbandonCh())

		index, result, err := query(ws, state)
		// Always set a non-zero index. Generally we expect the index
		// to be set to Raft index which can never be 0. If the query
		// returned no results we expect it to be set to the max index of the table,
		// however we can't guarantee this always happens.
		// To prevent a client from accidentally performing many non-blocking queries
		// (which causes lots of unnecessary load), we always set a default value of 1.
		// This is sufficient to prevent the unnecessary load in most cases.
		if index < 1 {
			index = 1
		}

		switch {
		case errors.Is(err, ErrorNotFound):
			// if minIndex is 0 then we should never block but we
			// also should not propagate the error
			if minIndex == 0 {
				return index, result, nil
			}

			// update the min index if the previous result was not found. This
			// is an attempt to not return data unnecessarily when we end up
			// watching the root of a memdb Radix tree because the data being
			// watched doesn't exist yet.
			if notFound {
				minIndex = index
			}

			notFound = true
		case err != nil:
			return index, result, err
		}

		// when enabled we can prevent sending back data that hasn't changed.
		if suppressSpuriousWakeup {
			newHash, err := hashstructure_v2.Hash(result, hashstructure_v2.FormatV2, nil)
			if err != nil {
				return index, result, fmt.Errorf("error hashing data for spurious wakeup suppression: %w", err)
			}

			// set minIndex to the returned index to prevent sending back identical data
			if ranOnce && priorHash == newHash {
				minIndex = index
			}
			ranOnce = true
			priorHash = newHash
		}

		// one final check if we should be considered unblocked and
		// return the value. Some conditions in the switch above
		// alter the minIndex and prevent this return if it would
		// be desirable. One such case is when the actual data has
		// not changed since the last round through the query and
		// we would rather not do any further processing for unchanged
		// data. This mostly protects against watches for data that
		// doesn't exist from return the non-existant value constantly.
		if index > minIndex {
			return index, result, nil
		}

		// Block until something changes. Because we have added the state
		// stores AbandonCh to this watch set, a snapshot restore will
		// cause things to unblock in addition to changes to the actual
		// queried data.
		if err := ws.WatchCtx(ctx); err != nil {
			// exit if the context was cancelled
			return index, result, nil
		}

		select {
		case <-state.AbandonCh():
			return index, result, nil
		default:
		}
	}
}

// ServerLocalNotify will watch for changes in the State Store using the provided
// query function and invoke the notify callback whenever the results of that query
// function have changed. This function will return an error if parameter validations
// fail but otherwise the background go routine to process the notifications will
// be spawned and nil will be returned. Just like ServerLocalBlockingQuery this makes
// use of Go Generics and for the same reasons as outlined in the documentation for
// that function.
func ServerLocalNotify[ResultType any, StoreType StateStore](
	ctx context.Context,
	correlationID string,
	getStore func() StoreType,
	query func(memdb.WatchSet, StoreType) (uint64, ResultType, error),
	notify func(ctx context.Context, correlationID string, result ResultType, err error),
) error {
	return serverLocalNotify(
		ctx,
		correlationID,
		getStore,
		query,
		notify,
		// Public callers should not need to know when the internal go routines are finished.
		// Being able to provide a done function to the internal version of this function is
		// to allow our tests to be more determinstic and to eliminate arbitrary sleeps.
		noopDone,
		// Public callers do not get to override the error backoff configuration. Internally
		// we want to allow for this to enable our unit tests to run much more quickly.
		defaultWaiter(),
	)
}

// serverLocalNotify is the internal version of ServerLocalNotify. It takes
// two additional arguments of the waiter to use and a function to call
// when the notification go routine has finished
func serverLocalNotify[ResultType any, StoreType StateStore](
	ctx context.Context,
	correlationID string,
	getStore func() StoreType,
	query func(memdb.WatchSet, StoreType) (uint64, ResultType, error),
	notify func(ctx context.Context, correlationID string, result ResultType, err error),
	done func(),
	waiter *retry.Waiter,
) error {
	if ctx == nil {
		return errNilContext
	}

	if getStore == nil {
		return errNilGetStore
	}

	if query == nil {
		return errNilQuery
	}

	if notify == nil {
		return errNilNotify
	}

	go serverLocalNotifyRoutine(
		ctx,
		correlationID,
		getStore,
		query,
		notify,
		done,
		waiter,
	)
	return nil
}

// serverLocalNotifyRoutine is the function intended to be run within a new
// go routine to process the updates. It will not check to ensure callbacks
// are non-nil nor perform other parameter validation. It is assumed that
// the in-package caller of this method will have already done that. It also
// takes the backoff waiter in as an argument so that unit tests within this
// package can override the default values that the exported ServerLocalNotify
// function would have set up.
func serverLocalNotifyRoutine[ResultType any, StoreType StateStore](
	ctx context.Context,
	correlationID string,
	getStore func() StoreType,
	query func(memdb.WatchSet, StoreType) (uint64, ResultType, error),
	notify func(ctx context.Context, correlationID string, result ResultType, err error),
	done func(),
	waiter *retry.Waiter,
) {
	defer done()

	var minIndex uint64

	for {
		// Check if the context has been cancelled. Do not issue
		// more queries if it has been cancelled.
		if ctx.Err() != nil {
			return
		}

		// Perform the blocking query
		index, result, err := ServerLocalBlockingQuery(ctx, getStore, minIndex, true, query)

		// Check if the context has been cancelled. If it has we should not send more
		// notifications.
		if ctx.Err() != nil {
			return
		}

		// Check the index to see if we should call notify
		if minIndex == 0 || minIndex < index {
			notify(ctx, correlationID, result, err)
			minIndex = index
		}

		// Handle errors with backoff. Badly behaved blocking calls that returned
		// a zero index are considered as failures since we need to not get stuck
		// in a busy loop.
		if err == nil && index > 0 {
			waiter.Reset()
		} else {
			if waiter.Wait(ctx) != nil {
				return
			}
		}

		// ensure we don't use zero indexes
		if err == nil && minIndex < 1 {
			minIndex = 1
		}
	}
}
