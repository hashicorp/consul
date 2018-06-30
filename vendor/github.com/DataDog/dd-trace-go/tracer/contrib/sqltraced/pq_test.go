package sqltraced

import (
	"log"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced/sqltest"
	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/lib/pq"
)

func TestPostgres(t *testing.T) {
	trc, transport := tracertest.GetTestTracer()
	db, err := OpenTraced(&pq.Driver{}, "postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable", "postgres-test", trc)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	testDB := &sqltest.DB{
		DB:         db,
		Tracer:     trc,
		Transport:  transport,
		DriverName: "postgres",
	}

	expectedSpan := &tracer.Span{
		Name:    "postgres.query",
		Service: "postgres-test",
		Type:    "sql",
	}
	expectedSpan.Meta = map[string]string{
		"db.user":  "postgres",
		"out.host": "127.0.0.1",
		"out.port": "55432",
		"db.name":  "postgres",
	}

	sqltest.AllSQLTests(t, testDB, expectedSpan)
}
