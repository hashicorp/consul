package gocqltrace

import (
	"context"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/gocql/gocql"
)

// To trace Cassandra commands, use our query wrapper TraceQuery.
func Example() {

	// Initialise a Cassandra session as usual, create a query.
	cluster := gocql.NewCluster("127.0.0.1")
	session, _ := cluster.CreateSession()
	query := session.Query("CREATE KEYSPACE if not exists trace WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1}")

	// Use context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/home")
	ctx := root.Context(context.Background())

	// Wrap the query to trace it and pass the context for inheritance
	tracedQuery := TraceQuery("ServiceName", tracer.DefaultTracer, query)
	tracedQuery.WithContext(ctx)

	// Execute your query as usual
	tracedQuery.Exec()
}
