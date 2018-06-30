// Package parsedsn provides functions to parse any kind of DSNs into a map[string]string
package parsedsn

import (
	"strings"

	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced/parsedsn/mysql"
	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced/parsedsn/pq"
)

// Postgres parses a postgres-type dsn into a map
func Postgres(dsn string) (map[string]string, error) {
	var err error
	meta := make(map[string]string)

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		dsn, err = pq.ParseURL(dsn)
		if err != nil {
			return nil, err
		}
	}

	if err := pq.ParseOpts(dsn, meta); err != nil {
		return nil, err
	}

	// Assure that we do not pass the user secret
	delete(meta, "password")

	return meta, nil
}

// MySQL parses a mysql-type dsn into a map
func MySQL(dsn string) (m map[string]string, err error) {
	var cfg *mysql.Config
	if cfg, err = mysql.ParseDSN(dsn); err == nil {
		addr := strings.Split(cfg.Addr, ":")
		m = map[string]string{
			"user":   cfg.User,
			"host":   addr[0],
			"port":   addr[1],
			"dbname": cfg.DBName,
		}
		return m, nil
	}
	return nil, err
}
