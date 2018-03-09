// Package sqlxtraced provides a traced version of the "jmoiron/sqlx" package
// For more information about the API, see https://godoc.org/github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced.
package sqlxtraced

import (
	"database/sql/driver"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced"
	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced/sqlutils"
	"github.com/jmoiron/sqlx"
)

// OpenTraced will first register the traced version of the `driver` if not yet registered and will then open a connection with it.
// This is usually the only function to use when there is no need for the granularity offered by Register and Open.
// The last argument is optional and allows you to pass a custom tracer.
func OpenTraced(driver driver.Driver, dataSourceName, service string, trcv ...*tracer.Tracer) (*sqlx.DB, error) {
	driverName := sqlutils.GetDriverName(driver)
	Register(driverName, driver, trcv...)
	return Open(driverName, dataSourceName, service)
}

// Register registers a traced version of `driver`.
func Register(driverName string, driver driver.Driver, trcv ...*tracer.Tracer) {
	sqltraced.Register(driverName, driver, trcv...)
}

// Open returns a traced version of *sqlx.DB.
func Open(driverName, dataSourceName, service string) (*sqlx.DB, error) {
	db, err := sqltraced.Open(driverName, dataSourceName, service)
	if err != nil {
		return nil, err
	}
	return sqlx.NewDb(db, driverName), err
}
