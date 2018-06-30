package gocql

import (
	"context"
	"net/http"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
)

const (
	debug          = false
	CASSANDRA_HOST = "127.0.0.1:59042"
)

func newCassandraCluster() *gocql.ClusterConfig {
	cluster := gocql.NewCluster(CASSANDRA_HOST)
	// the InitialHostLookup must be disabled in newer versions of
	// gocql otherwise "no connections were made when creating the session"
	// error is returned for Cassandra misconfiguration (that we don't need
	// since we're testing another behavior and not the client).
	// Check: https://github.com/gocql/gocql/issues/946
	cluster.DisableInitialHostLookup = true
	return cluster
}

// TestMain sets up the Keyspace and table if they do not exist
func TestMain(m *testing.M) {
	cluster := newCassandraCluster()
	session, _ := cluster.CreateSession()

	// Ensures test keyspace and table person exists.
	session.Query("CREATE KEYSPACE if not exists trace WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1}").Exec()
	session.Query("CREATE TABLE if not exists trace.person (name text PRIMARY KEY, age int, description text)").Exec()
	session.Query("INSERT INTO trace.person (name, age, description) VALUES ('Cassandra', 100, 'A cruel mistress')").Exec()

	m.Run()
}

func TestErrorWrapper(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	cluster := newCassandraCluster()
	session, _ := cluster.CreateSession()
	q := session.Query("CREATE KEYSPACE trace WITH REPLICATION = { 'class' : 'NetworkTopologyStrategy', 'datacenter1' : 1 };")
	err := TraceQuery("ServiceName", testTracer, q).Exec()

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	span := spans[0]

	assert.Equal(int32(span.Error), int32(1))
	assert.Equal(span.GetMeta("error.msg"), err.Error())
	assert.Equal(span.Name, ext.CassandraQuery)
	assert.Equal(span.Resource, "CREATE KEYSPACE trace WITH REPLICATION = { 'class' : 'NetworkTopologyStrategy', 'datacenter1' : 1 };")
	assert.Equal(span.Service, "ServiceName")
	assert.Equal(span.GetMeta(ext.CassandraConsistencyLevel), "4")
	assert.Equal(span.GetMeta(ext.CassandraPaginated), "false")

	// Not added in case of an error
	assert.Equal(span.GetMeta(ext.TargetHost), "")
	assert.Equal(span.GetMeta(ext.TargetPort), "")
	assert.Equal(span.GetMeta(ext.CassandraCluster), "")
	assert.Equal(span.GetMeta(ext.CassandraKeyspace), "")
}

func TestChildWrapperSpan(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	// Parent span
	ctx := context.Background()
	parentSpan := testTracer.NewChildSpanFromContext("parentSpan", ctx)
	ctx = tracer.ContextWithSpan(ctx, parentSpan)

	cluster := newCassandraCluster()
	session, _ := cluster.CreateSession()
	q := session.Query("SELECT * from trace.person")
	tq := TraceQuery("TestServiceName", testTracer, q)
	tq.WithContext(ctx).Exec()
	parentSpan.Finish()

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 2)

	var childSpan, pSpan *tracer.Span
	if spans[0].ParentID == spans[1].SpanID {
		childSpan = spans[0]
		pSpan = spans[1]
	} else {
		childSpan = spans[1]
		pSpan = spans[0]
	}
	assert.Equal(pSpan.Name, "parentSpan")
	assert.Equal(childSpan.ParentID, pSpan.SpanID)
	assert.Equal(childSpan.Name, ext.CassandraQuery)
	assert.Equal(childSpan.Resource, "SELECT * from trace.person")
	assert.Equal(childSpan.GetMeta(ext.CassandraKeyspace), "trace")
	assert.Equal(childSpan.GetMeta(ext.TargetPort), "59042")
	assert.Equal(childSpan.GetMeta(ext.TargetHost), "127.0.0.1")
	assert.Equal(childSpan.GetMeta(ext.CassandraCluster), "datacenter1")
}

// getTestTracer returns a Tracer with a DummyTransport
func getTestTracer() (*tracer.Tracer, *dummyTransport) {
	transport := &dummyTransport{}
	tracer := tracer.NewTracerTransport(transport)
	return tracer, transport
}

// dummyTransport is a transport that just buffers spans and encoding
type dummyTransport struct {
	traces   [][]*tracer.Span
	services map[string]tracer.Service
}

func (t *dummyTransport) SendTraces(traces [][]*tracer.Span) (*http.Response, error) {
	t.traces = append(t.traces, traces...)
	return nil, nil
}

func (t *dummyTransport) SendServices(services map[string]tracer.Service) (*http.Response, error) {
	t.services = services
	return nil, nil
}

func (t *dummyTransport) Traces() [][]*tracer.Span {
	traces := t.traces
	t.traces = nil
	return traces
}
func (t *dummyTransport) SetHeader(key, value string) {}
