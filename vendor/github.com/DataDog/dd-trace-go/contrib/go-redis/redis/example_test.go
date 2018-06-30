package redis_test

import (
	"context"
	"fmt"
	redistrace "github.com/DataDog/dd-trace-go/contrib/go-redis/redis"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/contrib/gin-gonic/gintrace"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"time"
)

// To start tracing Redis commands, use the NewTracedClient function to create a traced Redis clienty,
// passing in a service name of choice.
func Example() {
	opts := &redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "", // no password set
		DB:       0,  // use default db
	}
	c := redistrace.NewTracedClient(opts, tracer.DefaultTracer, "my-redis-backend")
	// Emit spans per command by using your Redis connection as usual
	c.Set("test_key", "test_value", 0)

	// Use a context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/home")
	ctx := root.Context(context.Background())

	// When set with a context, the traced client will emit a span inheriting from 'parent.request'
	c.SetContext(ctx)
	c.Set("food", "cheese", 0)
	root.Finish()

	// Contexts can be easily passed between Datadog integrations
	r := gin.Default()
	r.Use(gintrace.Middleware("web-admin"))
	client := redistrace.NewTracedClient(opts, tracer.DefaultTracer, "redis-img-backend")

	r.GET("/user/settings/:id", func(ctx *gin.Context) {
		// create a span that is a child of your http request
		client.SetContext(ctx)
		client.Get(fmt.Sprintf("cached_user_details_%s", ctx.Param("id")))
	})
}

// You can also trace Redis Pipelines
func Example_pipeline() {
	opts := &redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "", // no password set
		DB:       0,  // use default db
	}
	c := redistrace.NewTracedClient(opts, tracer.DefaultTracer, "my-redis-backend")
	// pipe is a TracedPipeliner
	pipe := c.Pipeline()
	pipe.Incr("pipeline_counter")
	pipe.Expire("pipeline_counter", time.Hour)

	pipe.Exec()
}

func ExampleNewTracedClient() {
	opts := &redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "", // no password set
		DB:       0,  // use default db
	}
	c := redistrace.NewTracedClient(opts, tracer.DefaultTracer, "my-redis-backend")
	// Emit spans per command by using your Redis connection as usual
	c.Set("test_key", "test_value", 0)

	// Use a context to pass information down the call chain
	root := tracer.NewRootSpan("parent.request", "web", "/home")
	ctx := root.Context(context.Background())

	// When set with a context, the traced client will emit a span inheriting from 'parent.request'
	c.SetContext(ctx)
	c.Set("food", "cheese", 0)
	root.Finish()
}
