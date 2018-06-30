// Package goredistrace provides tracing for the go-redis Redis client (https://github.com/go-redis/redis)
package goredistrace

import (
	"bytes"
	"context"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/go-redis/redis"
	"strconv"
	"strings"
)

// TracedClient is used to trace requests to a redis server.
type TracedClient struct {
	*redis.Client
	traceParams traceParams
}

// TracedPipeline is used to trace pipelines executed on a redis server.
type TracedPipeliner struct {
	redis.Pipeliner
	traceParams traceParams
}

type traceParams struct {
	host    string
	port    string
	db      string
	service string
	tracer  *tracer.Tracer
}

// NewTracedClient takes a Client returned by redis.NewClient and configures it to emit spans under the given service name
func NewTracedClient(opt *redis.Options, t *tracer.Tracer, service string) *TracedClient {
	var host, port string
	addr := strings.Split(opt.Addr, ":")
	if len(addr) == 2 && addr[1] != "" {
		port = addr[1]
	} else {
		port = "6379"
	}
	host = addr[0]
	db := strconv.Itoa(opt.DB)

	client := redis.NewClient(opt)
	t.SetServiceInfo(service, "redis", ext.AppTypeDB)
	tc := &TracedClient{
		client,
		traceParams{
			host,
			port,
			db,
			service,
			t},
	}

	tc.Client.WrapProcess(createWrapperFromClient(tc))
	return tc
}

// Pipeline creates a TracedPipeline from a TracedClient
func (c *TracedClient) Pipeline() *TracedPipeliner {
	return &TracedPipeliner{
		c.Client.Pipeline(),
		c.traceParams,
	}
}

// ExecWithContext calls Pipeline.Exec(). It ensures that the resulting Redis calls
// are traced, and that emitted spans are children of the given Context
func (c *TracedPipeliner) ExecWithContext(ctx context.Context) ([]redis.Cmder, error) {
	span := c.traceParams.tracer.NewChildSpanFromContext("redis.command", ctx)
	span.Service = c.traceParams.service

	span.SetMeta("out.host", c.traceParams.host)
	span.SetMeta("out.port", c.traceParams.port)
	span.SetMeta("out.db", c.traceParams.db)

	cmds, err := c.Pipeliner.Exec()
	if err != nil {
		span.SetError(err)
	}
	span.Resource = String(cmds)
	span.SetMeta("redis.pipeline_length", strconv.Itoa(len(cmds)))
	span.Finish()
	return cmds, err
}

// Exec calls Pipeline.Exec() ensuring that the resulting Redis calls are traced
func (c *TracedPipeliner) Exec() ([]redis.Cmder, error) {
	span := c.traceParams.tracer.NewRootSpan("redis.command", c.traceParams.service, "redis")

	span.SetMeta("out.host", c.traceParams.host)
	span.SetMeta("out.port", c.traceParams.port)
	span.SetMeta("out.db", c.traceParams.db)

	cmds, err := c.Pipeliner.Exec()
	if err != nil {
		span.SetError(err)
	}
	span.Resource = String(cmds)
	span.SetMeta("redis.pipeline_length", strconv.Itoa(len(cmds)))
	span.Finish()
	return cmds, err
}

// String returns a string representation of a slice of redis Commands, separated by newlines
func String(cmds []redis.Cmder) string {
	var b bytes.Buffer
	for _, cmd := range cmds {
		b.WriteString(cmd.String())
		b.WriteString("\n")
	}
	return b.String()
}

// SetContext sets a context on a TracedClient. Use it to ensure that emitted spans have the correct parent
func (c *TracedClient) SetContext(ctx context.Context) {
	c.Client = c.Client.WithContext(ctx)
}

// createWrapperFromClient wraps tracing into redis.Process().
func createWrapperFromClient(tc *TracedClient) func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
	return func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			ctx := tc.Client.Context()

			var resource string
			resource = strings.Split(cmd.String(), " ")[0]
			args_length := len(strings.Split(cmd.String(), " ")) - 1
			span := tc.traceParams.tracer.NewChildSpanFromContext("redis.command", ctx)

			span.Service = tc.traceParams.service
			span.Resource = resource

			span.SetMeta("redis.raw_command", cmd.String())
			span.SetMeta("redis.args_length", strconv.Itoa(args_length))
			span.SetMeta("out.host", tc.traceParams.host)
			span.SetMeta("out.port", tc.traceParams.port)
			span.SetMeta("out.db", tc.traceParams.db)

			err := oldProcess(cmd)
			if err != nil {
				span.SetError(err)
			}
			span.Finish()
			return err
		}
	}
}
