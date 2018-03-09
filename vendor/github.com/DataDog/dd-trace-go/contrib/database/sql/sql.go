// Package sqltraced provides a traced version of any driver implementing the database/sql/driver interface.
// To trace jmoiron/sqlx, see https://godoc.org/github.com/DataDog/dd-trace-go/tracer/contrib/sqlxtraced.
package sql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	log "github.com/cihub/seelog"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced/sqlutils"
	"github.com/DataDog/dd-trace-go/tracer/ext"
)

// OpenTraced will first register the traced version of the `driver` if not yet registered and will then open a connection with it.
// This is usually the only function to use when there is no need for the granularity offered by Register and Open.
// The last parameter is optional and enables you to use a custom tracer.
func OpenTraced(driver driver.Driver, dataSourceName, service string, trcv ...*tracer.Tracer) (*sql.DB, error) {
	driverName := sqlutils.GetDriverName(driver)
	Register(driverName, driver, trcv...)
	return Open(driverName, dataSourceName, service)
}

// Register takes a driver and registers a traced version of this one.
// The last parameter is optional and enables you to use a custom tracer.
func Register(driverName string, driver driver.Driver, trcv ...*tracer.Tracer) {
	if driver == nil {
		log.Error("RegisterTracedDriver: driver is nil")
		return
	}
	var trc *tracer.Tracer
	if len(trcv) == 0 || (len(trcv) > 0 && trcv[0] == nil) {
		trc = tracer.DefaultTracer
	} else {
		trc = trcv[0]
	}

	tracedDriverName := sqlutils.GetTracedDriverName(driverName)
	if !stringInSlice(sql.Drivers(), tracedDriverName) {
		td := tracedDriver{
			Driver:     driver,
			tracer:     trc,
			driverName: driverName,
		}
		sql.Register(tracedDriverName, td)
		log.Infof("Register %s driver", tracedDriverName)
	} else {
		log.Warnf("RegisterTracedDriver: %s already registered", tracedDriverName)
	}
}

// Open extends the usual API of sql.Open so you can specify the name of the service
// under which the traces will appear in the datadog app.
func Open(driverName, dataSourceName, service string) (*sql.DB, error) {
	tracedDriverName := sqlutils.GetTracedDriverName(driverName)
	// The service is passed through the DSN
	dsnAndService := newDSNAndService(dataSourceName, service)
	return sql.Open(tracedDriverName, dsnAndService)
}

// tracedDriver is a driver we use as a middleware between the database/sql package
// and the driver chosen (e.g. mysql, postgresql...).
// It implements the driver.Driver interface and add the tracing features on top
// of the driver's methods.
type tracedDriver struct {
	driver.Driver
	tracer     *tracer.Tracer
	driverName string
}

// Open returns a tracedConn so that we can pass all the info we get from the DSN
// all along the tracing
func (td tracedDriver) Open(dsnAndService string) (c driver.Conn, err error) {
	var meta map[string]string
	var conn driver.Conn

	dsn, service := parseDSNAndService(dsnAndService)

	// Register the service to Datadog tracing API
	td.tracer.SetServiceInfo(service, td.driverName, ext.AppTypeDB)

	// Get all kinds of information from the DSN
	meta, err = parseDSN(td.driverName, dsn)
	if err != nil {
		return nil, err
	}

	conn, err = td.Driver.Open(dsn)
	if err != nil {
		return nil, err
	}

	ti := traceInfo{
		tracer:     td.tracer,
		driverName: td.driverName,
		service:    service,
		meta:       meta,
	}
	return &tracedConn{conn, ti}, err
}

// traceInfo stores all information relative to the tracing
type traceInfo struct {
	tracer     *tracer.Tracer
	driverName string
	service    string
	resource   string
	meta       map[string]string
}

func (ti traceInfo) getSpan(ctx context.Context, resource string, query ...string) *tracer.Span {
	name := fmt.Sprintf("%s.%s", ti.driverName, "query")
	span := ti.tracer.NewChildSpanFromContext(name, ctx)
	span.Type = ext.SQLType
	span.Service = ti.service
	span.Resource = resource
	if len(query) > 0 {
		span.Resource = query[0]
		span.SetMeta(ext.SQLQuery, query[0])
	}
	for k, v := range ti.meta {
		span.SetMeta(k, v)
	}
	return span
}

type tracedConn struct {
	driver.Conn
	traceInfo
}

func (tc tracedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	span := tc.getSpan(ctx, "Begin")
	defer func() {
		span.SetError(err)
		span.Finish()
	}()
	if connBeginTx, ok := tc.Conn.(driver.ConnBeginTx); ok {
		tx, err = connBeginTx.BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}

		return tracedTx{tx, tc.traceInfo, ctx}, nil
	}

	tx, err = tc.Conn.Begin()
	if err != nil {
		return nil, err
	}

	return tracedTx{tx, tc.traceInfo, ctx}, nil
}

func (tc tracedConn) PrepareContext(ctx context.Context, query string) (stmt driver.Stmt, err error) {
	span := tc.getSpan(ctx, "Prepare", query)
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	// Check if the driver implements PrepareContext
	if connPrepareCtx, ok := tc.Conn.(driver.ConnPrepareContext); ok {
		stmt, err := connPrepareCtx.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		return tracedStmt{stmt, tc.traceInfo, ctx, query}, nil
	}

	// If the driver does not implement PrepareContex (lib/pq for example)
	stmt, err = tc.Prepare(query)
	if err != nil {
		return nil, err
	}
	return tracedStmt{stmt, tc.traceInfo, ctx, query}, nil
}

func (tc tracedConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if execer, ok := tc.Conn.(driver.Execer); ok {
		return execer.Exec(query, args)
	}

	return nil, driver.ErrSkip
}

func (tc tracedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (r driver.Result, err error) {
	span := tc.getSpan(ctx, "Exec", query)
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	if execContext, ok := tc.Conn.(driver.ExecerContext); ok {
		res, err := execContext.ExecContext(ctx, query, args)
		if err != nil {
			return nil, err
		}

		return res, nil
	}

	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}

	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return tc.Exec(query, dargs)
}

// tracedConn has a Ping method in order to implement the pinger interface
func (tc tracedConn) Ping(ctx context.Context) (err error) {
	span := tc.getSpan(ctx, "Ping")
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	if pinger, ok := tc.Conn.(driver.Pinger); ok {
		err = pinger.Ping(ctx)
	}

	return err
}

func (tc tracedConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	if queryer, ok := tc.Conn.(driver.Queryer); ok {
		return queryer.Query(query, args)
	}

	return nil, driver.ErrSkip
}

func (tc tracedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (rows driver.Rows, err error) {
	span := tc.getSpan(ctx, "Query", query)
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	if queryerContext, ok := tc.Conn.(driver.QueryerContext); ok {
		rows, err := queryerContext.QueryContext(ctx, query, args)
		if err != nil {
			return nil, err
		}

		return rows, nil
	}

	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}

	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return tc.Query(query, dargs)
}

// tracedTx is a traced version of sql.Tx
type tracedTx struct {
	driver.Tx
	traceInfo
	ctx context.Context
}

// Commit sends a span at the end of the transaction
func (t tracedTx) Commit() (err error) {
	span := t.getSpan(t.ctx, "Commit")
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	return t.Tx.Commit()
}

// Rollback sends a span if the connection is aborted
func (t tracedTx) Rollback() (err error) {
	span := t.getSpan(t.ctx, "Rollback")
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	return t.Tx.Rollback()
}

// tracedStmt is traced version of sql.Stmt
type tracedStmt struct {
	driver.Stmt
	traceInfo
	ctx   context.Context
	query string
}

// Close sends a span before closing a statement
func (s tracedStmt) Close() (err error) {
	span := s.getSpan(s.ctx, "Close")
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	return s.Stmt.Close()
}

// ExecContext is needed to implement the driver.StmtExecContext interface
func (s tracedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (res driver.Result, err error) {
	span := s.getSpan(s.ctx, "Exec", s.query)
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	if stmtExecContext, ok := s.Stmt.(driver.StmtExecContext); ok {
		res, err = stmtExecContext.ExecContext(ctx, args)
		if err != nil {
			return nil, err
		}

		return res, nil
	}

	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}

	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return s.Exec(dargs)
}

// QueryContext is needed to implement the driver.StmtQueryContext interface
func (s tracedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (rows driver.Rows, err error) {
	span := s.getSpan(s.ctx, "Query", s.query)
	defer func() {
		span.SetError(err)
		span.Finish()
	}()

	if stmtQueryContext, ok := s.Stmt.(driver.StmtQueryContext); ok {
		rows, err = stmtQueryContext.QueryContext(ctx, args)
		if err != nil {
			return nil, err
		}

		return rows, nil
	}

	// Fallback implementation
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}

	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return s.Query(dargs)
}
