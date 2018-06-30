// Package redigo provides tracing for the Redigo Redis client (https://github.com/garyburd/redigo)
package redigo

import (
	"bytes"
	"context"
	"fmt"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	redis "github.com/garyburd/redigo/redis"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// TracedConn is an implementation of the redis.Conn interface that supports tracing
type TracedConn struct {
	redis.Conn
	p traceParams
}

// traceParams contains fields and metadata useful for command tracing
type traceParams struct {
	tracer  *tracer.Tracer
	service string
	network string
	host    string
	port    string
}

// TracedDial takes a Conn returned by redis.Dial and configures it to emit spans with the given service name
func TracedDial(service string, tracer *tracer.Tracer, network, address string, options ...redis.DialOption) (redis.Conn, error) {
	c, err := redis.Dial(network, address, options...)
	addr := strings.Split(address, ":")
	var host, port string
	if len(addr) == 2 && addr[1] != "" {
		port = addr[1]
	} else {
		port = "6379"
	}
	host = addr[0]
	tracer.SetServiceInfo(service, "redis", ext.AppTypeDB)
	tc := TracedConn{c, traceParams{tracer, service, network, host, port}}
	return tc, err
}

// TracedDialURL takes a Conn returned by redis.DialURL and configures it to emit spans with the given service name
func TracedDialURL(service string, tracer *tracer.Tracer, rawurl string, options ...redis.DialOption) (redis.Conn, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return TracedConn{}, err
	}

	// Getting host and port, usind code from https://github.com/garyburd/redigo/blob/master/redis/conn.go#L226
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
		port = "6379"
	}
	if host == "" {
		host = "localhost"
	}
	// Set in redis.DialUrl source code
	network := "tcp"
	c, err := redis.DialURL(rawurl, options...)
	tc := TracedConn{c, traceParams{tracer, service, network, host, port}}
	return tc, err
}

// NewChildSpan creates a span inheriting from the given context. It adds to the span useful metadata about the traced Redis connection
func (tc TracedConn) NewChildSpan(ctx context.Context) *tracer.Span {
	span := tc.p.tracer.NewChildSpanFromContext("redis.command", ctx)
	span.Service = tc.p.service
	span.SetMeta("out.network", tc.p.network)
	span.SetMeta("out.port", tc.p.port)
	span.SetMeta("out.host", tc.p.host)
	return span
}

// Do wraps redis.Conn.Do. It sends a command to the Redis server and returns the received reply.
// In the process it emits a span containing key information about the command sent.
// When passed a context.Context as the final argument, Do will ensure that any span created
// inherits from this context. The rest of the arguments are passed through to the Redis server unchanged
func (tc TracedConn) Do(commandName string, args ...interface{}) (reply interface{}, err error) {
	var ctx context.Context
	var ok bool
	if len(args) > 0 {
		ctx, ok = args[len(args)-1].(context.Context)
		if ok {
			args = args[:len(args)-1]
		}
	}

	span := tc.NewChildSpan(ctx)
	defer func() {
		if err != nil {
			span.SetError(err)
		}
		span.Finish()
	}()

	span.SetMeta("redis.args_length", strconv.Itoa(len(args)))

	if len(commandName) > 0 {
		span.Resource = commandName
	} else {
		// When the command argument to the Do method is "", then the Do method will flush the output buffer
		// See https://godoc.org/github.com/garyburd/redigo/redis#hdr-Pipelining
		span.Resource = "redigo.Conn.Flush"
	}
	var b bytes.Buffer
	b.WriteString(commandName)
	for _, arg := range args {
		b.WriteString(" ")
		switch arg := arg.(type) {
		case string:
			b.WriteString(arg)
		case int:
			b.WriteString(strconv.Itoa(arg))
		case int32:
			b.WriteString(strconv.FormatInt(int64(arg), 10))
		case int64:
			b.WriteString(strconv.FormatInt(arg, 10))
		case fmt.Stringer:
			b.WriteString(arg.String())
		}
	}
	span.SetMeta("redis.raw_command", b.String())
	return tc.Conn.Do(commandName, args...)
}
