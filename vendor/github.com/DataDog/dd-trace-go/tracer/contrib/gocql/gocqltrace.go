// Package gocqltrace provides tracing for the Cassandra Gocql client (https://github.com/gocql/gocql)
package gocqltrace

import (
	"context"
	"strconv"
	"strings"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"

	"github.com/gocql/gocql"
)

// TracedQuery inherits from gocql.Query, it keeps the tracer and the context.
type TracedQuery struct {
	*gocql.Query
	p            traceParams
	traceContext context.Context
}

// TracedIter inherits from gocql.Iter and contains a span.
type TracedIter struct {
	*gocql.Iter
	span *tracer.Span
}

// traceParams containes fields and metadata useful for command tracing
type traceParams struct {
	tracer      *tracer.Tracer
	service     string
	keyspace    string
	paginated   string
	consistancy string
	query       string
}

// TraceQuery wraps a gocql.Query into a TracedQuery
func TraceQuery(service string, tracer *tracer.Tracer, q *gocql.Query) *TracedQuery {
	stringQuery := `"` + strings.SplitN(q.String(), "\"", 3)[1] + `"`
	stringQuery, err := strconv.Unquote(stringQuery)
	if err != nil {
		// An invalid string, so that the trace is not dropped
		// due to having an empty resource
		stringQuery = "_"
	}

	tq := &TracedQuery{q, traceParams{tracer, service, "", "false", strconv.Itoa(int(q.GetConsistency())), stringQuery}, context.Background()}
	tracer.SetServiceInfo(service, ext.CassandraType, ext.AppTypeDB)
	return tq
}

// WithContext rewrites the original function so that ctx can be used for inheritance
func (tq *TracedQuery) WithContext(ctx context.Context) *TracedQuery {
	tq.traceContext = ctx
	tq.Query.WithContext(ctx)
	return tq
}

// PageState rewrites the original function so that spans are aware of the change.
func (tq *TracedQuery) PageState(state []byte) *TracedQuery {
	tq.p.paginated = "true"
	tq.Query = tq.Query.PageState(state)
	return tq
}

// NewChildSpan creates a new span from the traceParams and the context.
func (tq *TracedQuery) NewChildSpan(ctx context.Context) *tracer.Span {
	span := tq.p.tracer.NewChildSpanFromContext(ext.CassandraQuery, ctx)
	span.Type = ext.CassandraType
	span.Service = tq.p.service
	span.Resource = tq.p.query
	span.SetMeta(ext.CassandraPaginated, tq.p.paginated)
	span.SetMeta(ext.CassandraKeyspace, tq.p.keyspace)
	return span
}

// Exec is rewritten so that it passes by our custom Iter
func (tq *TracedQuery) Exec() error {
	return tq.Iter().Close()
}

// MapScan wraps in a span query.MapScan call.
func (tq *TracedQuery) MapScan(m map[string]interface{}) error {
	span := tq.NewChildSpan(tq.traceContext)
	defer span.Finish()
	err := tq.Query.MapScan(m)
	if err != nil {
		span.SetError(err)
	}
	return err
}

// Scan wraps in a span query.Scan call.
func (tq *TracedQuery) Scan(dest ...interface{}) error {
	span := tq.NewChildSpan(tq.traceContext)
	defer span.Finish()
	err := tq.Query.Scan(dest...)
	if err != nil {
		span.SetError(err)
	}
	return err
}

// ScanCAS wraps in a span query.ScanCAS call.
func (tq *TracedQuery) ScanCAS(dest ...interface{}) (applied bool, err error) {
	span := tq.NewChildSpan(tq.traceContext)
	defer span.Finish()
	applied, err = tq.Query.ScanCAS(dest...)
	if err != nil {
		span.SetError(err)
	}
	return applied, err
}

// Iter starts a new span at query.Iter call.
func (tq *TracedQuery) Iter() *TracedIter {
	span := tq.NewChildSpan(tq.traceContext)
	iter := tq.Query.Iter()
	span.SetMeta(ext.CassandraRowCount, strconv.Itoa(iter.NumRows()))
	span.SetMeta(ext.CassandraConsistencyLevel, strconv.Itoa(int(tq.GetConsistency())))

	columns := iter.Columns()
	if len(columns) > 0 {
		span.SetMeta(ext.CassandraKeyspace, columns[0].Keyspace)
	} else {
	}
	tIter := &TracedIter{iter, span}
	if tIter.Host() != nil {
		tIter.span.SetMeta(ext.TargetHost, tIter.Iter.Host().HostID())
		tIter.span.SetMeta(ext.TargetPort, strconv.Itoa(tIter.Iter.Host().Port()))
		tIter.span.SetMeta(ext.CassandraCluster, tIter.Iter.Host().DataCenter())

	}
	return tIter
}

// Close closes the TracedIter and finish the span created on Iter call.
func (tIter *TracedIter) Close() error {
	err := tIter.Iter.Close()
	if err != nil {
		tIter.span.SetError(err)
	}
	tIter.span.Finish()
	return err
}
