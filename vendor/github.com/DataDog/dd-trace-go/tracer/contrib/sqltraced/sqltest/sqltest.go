// Package sqltest is used for testing sql packages
package sqltest

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/stretchr/testify/assert"
)

// setupTestCase initializes MySQL or Postgres databases and returns a
// teardown function that must be executed via `defer`
func setupTestCase(t *testing.T, db *DB) func(t *testing.T, db *DB) {
	// creates the database
	db.Exec("DROP TABLE IF EXISTS city")
	db.Exec("CREATE TABLE city (id integer NOT NULL DEFAULT '0', name text)")

	// Empty the tracer
	db.Tracer.ForceFlush()
	db.Transport.Traces()

	return func(t *testing.T, db *DB) {
		// drop the table
		db.Exec("DROP TABLE city")
	}
}

// AllSQLTests applies a sequence of unit tests to check the correct tracing of sql features.
func AllSQLTests(t *testing.T, db *DB, expectedSpan *tracer.Span) {
	// database setup and cleanup
	tearDown := setupTestCase(t, db)
	defer tearDown(t, db)

	testDB(t, db, expectedSpan)
	testStatement(t, db, expectedSpan)
	testTransaction(t, db, expectedSpan)
}

func testDB(t *testing.T, db *DB, expectedSpan *tracer.Span) {
	assert := assert.New(t)
	const query = "SELECT id, name FROM city LIMIT 5"

	// Test db.Ping
	err := db.Ping()
	assert.Equal(nil, err)

	db.Tracer.ForceFlush()
	traces := db.Transport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	actualSpan := spans[0]
	pingSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	pingSpan.Resource = "Ping"
	tracertest.CompareSpan(t, pingSpan, actualSpan)

	// Test db.Query
	rows, err := db.Query(query)
	defer rows.Close()
	assert.Equal(nil, err)

	db.Tracer.ForceFlush()
	traces = db.Transport.Traces()
	assert.Len(traces, 1)
	spans = traces[0]
	assert.Len(spans, 1)

	actualSpan = spans[0]
	querySpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	querySpan.Resource = query
	querySpan.SetMeta("sql.query", query)
	tracertest.CompareSpan(t, querySpan, actualSpan)
	delete(expectedSpan.Meta, "sql.query")
}

func testStatement(t *testing.T, db *DB, expectedSpan *tracer.Span) {
	assert := assert.New(t)
	query := "INSERT INTO city(name) VALUES(%s)"
	switch db.DriverName {
	case "postgres":
		query = fmt.Sprintf(query, "$1")
	case "mysql":
		query = fmt.Sprintf(query, "?")
	}

	// Test TracedConn.PrepareContext
	stmt, err := db.Prepare(query)
	assert.Equal(nil, err)

	db.Tracer.ForceFlush()
	traces := db.Transport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	actualSpan := spans[0]
	prepareSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	prepareSpan.Resource = query
	prepareSpan.SetMeta("sql.query", query)
	tracertest.CompareSpan(t, prepareSpan, actualSpan)
	delete(expectedSpan.Meta, "sql.query")

	// Test Exec
	_, err2 := stmt.Exec("New York")
	assert.Equal(nil, err2)

	db.Tracer.ForceFlush()
	traces = db.Transport.Traces()
	assert.Len(traces, 1)
	spans = traces[0]
	assert.Len(spans, 1)
	actualSpan = spans[0]

	execSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	execSpan.Resource = query
	execSpan.SetMeta("sql.query", query)
	tracertest.CompareSpan(t, execSpan, actualSpan)
	delete(expectedSpan.Meta, "sql.query")
}

func testTransaction(t *testing.T, db *DB, expectedSpan *tracer.Span) {
	assert := assert.New(t)
	query := "INSERT INTO city(name) VALUES('New York')"

	// Test Begin
	tx, err := db.Begin()
	assert.Equal(nil, err)

	db.Tracer.ForceFlush()
	traces := db.Transport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	actualSpan := spans[0]
	beginSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	beginSpan.Resource = "Begin"
	tracertest.CompareSpan(t, beginSpan, actualSpan)

	// Test Rollback
	err = tx.Rollback()
	assert.Equal(nil, err)

	db.Tracer.ForceFlush()
	traces = db.Transport.Traces()
	assert.Len(traces, 1)
	spans = traces[0]
	assert.Len(spans, 1)
	actualSpan = spans[0]
	rollbackSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	rollbackSpan.Resource = "Rollback"
	tracertest.CompareSpan(t, rollbackSpan, actualSpan)

	// Test Exec
	parentSpan := db.Tracer.NewRootSpan("test.parent", "test", "parent")
	ctx := tracer.ContextWithSpan(context.Background(), parentSpan)

	tx, err = db.BeginTx(ctx, nil)
	assert.Equal(nil, err)

	_, err = tx.ExecContext(ctx, query)
	assert.Equal(nil, err)

	err = tx.Commit()
	assert.Equal(nil, err)

	parentSpan.Finish() // need to do this else children are not flushed at all

	db.Tracer.ForceFlush()
	traces = db.Transport.Traces()
	assert.Len(traces, 1)
	spans = traces[0]
	assert.Len(spans, 4)

	for _, s := range spans {
		if s.Name == expectedSpan.Name && s.Resource == query {
			actualSpan = s
		}
	}

	assert.NotNil(actualSpan)
	execSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	execSpan.Resource = query
	execSpan.SetMeta("sql.query", query)
	tracertest.CompareSpan(t, execSpan, actualSpan)
	delete(expectedSpan.Meta, "sql.query")

	for _, s := range spans {
		if s.Name == expectedSpan.Name && s.Resource == "Commit" {
			actualSpan = s
		}
	}

	assert.NotNil(actualSpan)
	commitSpan := tracertest.CopySpan(expectedSpan, db.Tracer)
	commitSpan.Resource = "Commit"
	tracertest.CompareSpan(t, commitSpan, actualSpan)
}

// DB is a struct dedicated for testing
type DB struct {
	*sql.DB
	Tracer     *tracer.Tracer
	Transport  *tracertest.DummyTransport
	DriverName string
}
