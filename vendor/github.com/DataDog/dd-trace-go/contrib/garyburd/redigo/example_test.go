package redigo_test

import (
	"context"
	redigotrace "github.com/DataDog/dd-trace-go/contrib/garyburd/redigo"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/garyburd/redigo/redis"
)

// To start tracing Redis commands, use the TracedDial function to create a connection,
// passing in a service name of choice.
func Example() {
	c, _ := redigotrace.TracedDial("my-redis-backend", tracer.DefaultTracer, "tcp", "127.0.0.1:6379")

	// Emit spans per command by using your Redis connection as usual
	c.Do("SET", "vehicle", "truck")

	// Use a context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/home")
	ctx := root.Context(context.Background())

	// When passed a context as the final argument, c.Do will emit a span inheriting from 'parent.request'
	c.Do("SET", "food", "cheese", ctx)
	root.Finish()
}

func ExampleTracedConn() {
	c, _ := redigotrace.TracedDial("my-redis-backend", tracer.DefaultTracer, "tcp", "127.0.0.1:6379")

	// Emit spans per command by using your Redis connection as usual
	c.Do("SET", "vehicle", "truck")

	// Use a context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/home")
	ctx := root.Context(context.Background())

	// When passed a context as the final argument, c.Do will emit a span inheriting from 'parent.request'
	c.Do("SET", "food", "cheese", ctx)
	root.Finish()
}

// Alternatively, provide a redis URL to the TracedDialURL function
func Example_dialURL() {
	c, _ := redigotrace.TracedDialURL("my-redis-backend", tracer.DefaultTracer, "redis://127.0.0.1:6379")
	c.Do("SET", "vehicle", "truck")
}

// When using a redigo Pool, set your Dial function to return a traced connection
func Example_pool() {
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redigotrace.TracedDial("my-redis-backend", tracer.DefaultTracer, "tcp", "127.0.0.1:6379")
		},
	}

	c := pool.Get()

	c.Do("SET", " whiskey", " glass")
}
