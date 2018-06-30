package sql_test

import (
	"context"
	"log"

	sqltrace "github.com/DataDog/dd-trace-go/contrib/database/sql"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

// To trace the sql calls, you just need to open your sql.DB with OpenTraced.
// All calls through this sql.DB object will then be traced.
func Example() {
	// OpenTraced will first register a traced version of the driver and then will return the sql.DB object
	// that holds the connection with the database.
	// The third argument is used to specify the name of the service under which traces will appear in the Datadog app.
	db, err := sqltrace.OpenTraced(&pq.Driver{}, "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable", "web-backend")
	if err != nil {
		log.Fatal(err)
	}

	// All calls through the database/sql API will then be traced.
	rows, err := db.Query("SELECT name FROM users WHERE age=?", 27)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}

// If you want to link your db calls with existing traces, you need to use
// the context version of the database/sql API.
// Just make sure you are passing the parent span within the context.
func Example_context() {
	// OpenTraced will first register a traced version of the driver and then will return the sql.DB object
	// that holds the connection with the database.
	// The third argument is used to specify the name of the service under which traces will appear in the Datadog app.
	db, err := sqltrace.OpenTraced(&pq.Driver{}, "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable", "web-backend")
	if err != nil {
		log.Fatal(err)
	}

	// We create a parent span and put it within the context.
	span := tracer.NewRootSpan("postgres.parent", "web-backend", "query-parent")
	ctx := tracer.ContextWithSpan(context.Background(), span)

	// We need to use the context version of the database/sql API
	// in order to link this call with the parent span.
	db.PingContext(ctx)
	rows, _ := db.QueryContext(ctx, "SELECT * FROM city LIMIT 5")
	rows.Close()

	stmt, _ := db.PrepareContext(ctx, "INSERT INTO city(name) VALUES($1)")
	stmt.Exec("New York")
	stmt, _ = db.PrepareContext(ctx, "SELECT name FROM city LIMIT $1")
	rows, _ = stmt.Query(1)
	rows.Close()
	stmt.Close()

	tx, _ := db.BeginTx(ctx, nil)
	tx.ExecContext(ctx, "INSERT INTO city(name) VALUES('New York')")
	rows, _ = tx.QueryContext(ctx, "SELECT * FROM city LIMIT 5")
	rows.Close()
	stmt, _ = tx.PrepareContext(ctx, "SELECT name FROM city LIMIT $1")
	rows, _ = stmt.Query(1)
	rows.Close()
	stmt.Close()
	tx.Commit()

	// Calling span.Finish() will send the span into the tracer's buffer
	// and then being processed.
	span.Finish()
}

// You can trace all drivers implementing the database/sql/driver interface.
// For example, you can trace the go-sql-driver/mysql with the following code.
func Example_mySQL() {
	// OpenTraced will first register a traced version of the driver and then will return the sql.DB object
	// that holds the connection with the database.
	// The third argument is used to specify the name of the service under which traces will appear in the Datadog app.
	db, err := sqltrace.OpenTraced(&mysql.MySQLDriver{}, "user:password@/dbname", "web-backend")
	if err != nil {
		log.Fatal(err)
	}

	// All calls through the database/sql API will then be traced.
	rows, err := db.Query("SELECT name FROM users WHERE age=?", 27)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}

// OpenTraced will first register a traced version of the driver and then will return the sql.DB object
// that holds the connection with the database.
func ExampleOpenTraced() {
	// The first argument is a reference to the driver to trace.
	// The second argument is the dataSourceName.
	// The third argument is used to specify the name of the service under which traces will appear in the Datadog app.
	// The last argument allows you to specify a custom tracer to use for tracing.
	db, err := sqltrace.OpenTraced(&pq.Driver{}, "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable", "web-backend")
	if err != nil {
		log.Fatal(err)
	}

	// Use the database/sql API as usual and see traces appear in the Datadog app.
	rows, err := db.Query("SELECT name FROM users WHERE age=?", 27)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}

// You can use a custom tracer by passing it through the optional last argument of OpenTraced.
func ExampleOpenTraced_tracer() {
	// Create and customize a new tracer that will forward 50% of generated traces to the agent.
	// (useful to manage resource usage in high-throughput environments)
	trc := tracer.NewTracer()
	trc.SetSampleRate(0.5)

	// Pass your custom tracer through the last argument of OpenTraced to trace your db calls with it.
	db, err := sqltrace.OpenTraced(&pq.Driver{}, "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable", "web-backend", trc)
	if err != nil {
		log.Fatal(err)
	}

	// Use the database/sql API as usual and see traces appear in the Datadog app.
	rows, err := db.Query("SELECT name FROM users WHERE age=?", 27)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}

// If you need more granularity, you can register the traced driver seperately from the Open call.
func ExampleRegister() {
	// Register a traced version of your driver.
	sqltrace.Register("postgres", &pq.Driver{})

	// Returns a sql.DB object that holds the traced connection to the database.
	// Note: the sql.DB object returned by sql.Open will not be traced so make sure to use sql.Open.
	db, _ := sqltrace.Open("postgres", "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable", "web-backend")
	defer db.Close()

	// Use the database/sql API as usual and see traces appear in the Datadog app.
	rows, err := db.Query("SELECT name FROM users WHERE age=?", 27)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}

// You can use a custom tracer by passing it through the optional last argument of Register.
func ExampleRegister_tracer() {
	// Create and customize a new tracer that will forward 50% of generated traces to the agent.
	// (useful to manage resource usage in high-throughput environments)
	trc := tracer.NewTracer()
	trc.SetSampleRate(0.5)

	// Register a traced version of your driver and specify to use the previous tracer
	// to send the traces to the agent.
	sqltrace.Register("postgres", &pq.Driver{}, trc)

	// Returns a sql.DB object that holds the traced connection to the database.
	// Note: the sql.DB object returned by sql.Open will not be traced so make sure to use sql.Open.
	db, _ := sqltrace.Open("postgres", "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable", "web-backend")
	defer db.Close()
}
