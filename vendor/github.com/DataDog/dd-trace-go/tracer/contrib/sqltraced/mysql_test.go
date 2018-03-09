package sqltraced

import (
	"log"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/contrib/sqltraced/sqltest"
	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/go-sql-driver/mysql"
)

func TestMySQL(t *testing.T) {
	trc, transport := tracertest.GetTestTracer()
	db, err := OpenTraced(&mysql.MySQLDriver{}, "test:test@tcp(127.0.0.1:53306)/test", "mysql-test", trc)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	testDB := &sqltest.DB{
		DB:         db,
		Tracer:     trc,
		Transport:  transport,
		DriverName: "mysql",
	}

	expectedSpan := &tracer.Span{
		Name:    "mysql.query",
		Service: "mysql-test",
		Type:    "sql",
	}
	expectedSpan.Meta = map[string]string{
		"db.user":  "test",
		"out.host": "127.0.0.1",
		"out.port": "53306",
		"db.name":  "test",
	}

	sqltest.AllSQLTests(t, testDB, expectedSpan)
}
