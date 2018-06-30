package interceptor_test

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	testpb "github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc/test/otgrpc_testing"
	"github.com/opentracing/opentracing-go/mocktracer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	streamLength = 5
)

type testServer struct{}

func (s *testServer) UnaryCall(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	return &testpb.SimpleResponse{in.Payload}, nil
}

func (s *testServer) StreamingOutputCall(in *testpb.SimpleRequest, stream testpb.TestService_StreamingOutputCallServer) error {
	for i := 0; i < streamLength; i++ {
		if err := stream.Send(&testpb.SimpleResponse{in.Payload}); err != nil {
			return err
		}
	}
	return nil
}

func (s *testServer) StreamingInputCall(stream testpb.TestService_StreamingInputCallServer) error {
	sum := int32(0)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		sum += in.Payload
	}
	return stream.SendAndClose(&testpb.SimpleResponse{sum})
}

func (s *testServer) StreamingBidirectionalCall(stream testpb.TestService_StreamingBidirectionalCallServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = stream.Send(&testpb.SimpleResponse{in.Payload}); err != nil {
			return err
		}
	}
}

type env struct {
	unaryClientInt  grpc.UnaryClientInterceptor
	streamClientInt grpc.StreamClientInterceptor
	unaryServerInt  grpc.UnaryServerInterceptor
	streamServerInt grpc.StreamServerInterceptor
}

type test struct {
	t   *testing.T
	e   env
	srv *grpc.Server
	cc  *grpc.ClientConn
	c   testpb.TestServiceClient
}

func newTest(t *testing.T, e env) *test {
	te := &test{
		t: t,
		e: e,
	}

	// Set up the server.
	sOpts := []grpc.ServerOption{}
	if e.unaryServerInt != nil {
		sOpts = append(sOpts, grpc.UnaryInterceptor(e.unaryServerInt))
	}
	if e.streamServerInt != nil {
		sOpts = append(sOpts, grpc.StreamInterceptor(e.streamServerInt))
	}
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		te.t.Fatalf("Failed to listen: %v", err)
	}
	te.srv = grpc.NewServer(sOpts...)
	testpb.RegisterTestServiceServer(te.srv, &testServer{})
	go te.srv.Serve(lis)

	// Set up a connection to the server.
	cOpts := []grpc.DialOption{grpc.WithInsecure()}
	if e.unaryClientInt != nil {
		cOpts = append(cOpts, grpc.WithUnaryInterceptor(e.unaryClientInt))
	}
	if e.streamClientInt != nil {
		cOpts = append(cOpts, grpc.WithStreamInterceptor(e.streamClientInt))
	}
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		te.t.Fatalf("Failed to parse listener address: %v", err)
	}
	srvAddr := "localhost:" + port
	te.cc, err = grpc.Dial(srvAddr, cOpts...)
	if err != nil {
		te.t.Fatalf("Dial(%q) = %v", srvAddr, err)
	}
	te.c = testpb.NewTestServiceClient(te.cc)
	return te
}

func (te *test) tearDown() {
	te.cc.Close()
}

func assertChildParentSpans(t *testing.T, tracer *mocktracer.MockTracer) {
	spans := tracer.FinishedSpans()
	assert.Equal(t, 2, len(spans))
	if len(spans) != 2 {
		t.Fatalf("Incorrect span length")
	}
	parent := spans[1]
	child := spans[0]
	assert.Equal(t, child.ParentID, parent.Context().(mocktracer.MockSpanContext).SpanID)
}

func TestUnaryOpenTracing(t *testing.T) {
	tracer := mocktracer.New()
	e := env{
		unaryClientInt: otgrpc.OpenTracingClientInterceptor(tracer),
		unaryServerInt: otgrpc.OpenTracingServerInterceptor(tracer),
	}
	te := newTest(t, e)
	defer te.tearDown()

	payload := int32(0)
	resp, err := te.c.UnaryCall(context.Background(), &testpb.SimpleRequest{payload})
	if err != nil {
		t.Fatalf("Failed UnaryCall: %v", err)
	}
	assert.Equal(t, payload, resp.Payload)
	assertChildParentSpans(t, tracer)
}

func TestStreamingOutputCallOpenTracing(t *testing.T) {
	tracer := mocktracer.New()
	e := env{
		streamClientInt: otgrpc.OpenTracingStreamClientInterceptor(tracer),
		streamServerInt: otgrpc.OpenTracingStreamServerInterceptor(tracer),
	}
	te := newTest(t, e)
	defer te.tearDown()

	payload := int32(0)
	stream, err := te.c.StreamingOutputCall(context.Background(), &testpb.SimpleRequest{payload})
	if err != nil {
		t.Fatalf("Failed StreamingOutputCall: %v", err)
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed StreamingOutputCall: %v", err)
		}
		assert.Equal(t, payload, resp.Payload)
	}
	assertChildParentSpans(t, tracer)
}

func TestStreamingInputCallOpenTracing(t *testing.T) {
	tracer := mocktracer.New()
	e := env{
		streamClientInt: otgrpc.OpenTracingStreamClientInterceptor(tracer),
		streamServerInt: otgrpc.OpenTracingStreamServerInterceptor(tracer),
	}
	te := newTest(t, e)
	defer te.tearDown()

	payload := int32(1)
	stream, err := te.c.StreamingInputCall(context.Background())
	for i := 0; i < streamLength; i++ {
		if err = stream.Send(&testpb.SimpleRequest{payload}); err != nil {
			t.Fatalf("Failed StreamingInputCall: %v", err)
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("Failed StreamingInputCall: %v", err)
	}
	assert.Equal(t, streamLength*payload, resp.Payload)
	assertChildParentSpans(t, tracer)
}

func TestStreamingBidirectionalCallOpenTracing(t *testing.T) {
	tracer := mocktracer.New()
	e := env{
		streamClientInt: otgrpc.OpenTracingStreamClientInterceptor(tracer),
		streamServerInt: otgrpc.OpenTracingStreamServerInterceptor(tracer),
	}
	te := newTest(t, e)
	defer te.tearDown()

	payload := int32(0)
	stream, err := te.c.StreamingBidirectionalCall(context.Background())
	if err != nil {
		t.Fatalf("Failed StreamingInputCall: %v", err)
	}
	go func() {
		for i := 0; i < streamLength; i++ {
			if err := stream.Send(&testpb.SimpleRequest{payload}); err != nil {
				t.Fatalf("Failed StreamingInputCall: %v", err)
			}
		}
		stream.CloseSend()
	}()
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed StreamingOutputCall: %v", err)
		}
		assert.Equal(t, payload, resp.Payload)
	}
	assertChildParentSpans(t, tracer)
}

func TestStreamingContextCancellationOpenTracing(t *testing.T) {
	tracer := mocktracer.New()
	e := env{
		streamClientInt: otgrpc.OpenTracingStreamClientInterceptor(tracer),
		streamServerInt: otgrpc.OpenTracingStreamServerInterceptor(tracer),
	}
	te := newTest(t, e)
	defer te.tearDown()

	payload := int32(0)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := te.c.StreamingOutputCall(ctx, &testpb.SimpleRequest{payload})
	if err != nil {
		t.Fatalf("Failed StreamingOutputCall: %v", err)
	}
	cancel()
	time.Sleep(100 * time.Millisecond)
	spans := tracer.FinishedSpans()
	assert.Equal(t, 2, len(spans))
	if len(spans) != 2 {
		t.Fatalf("Incorrect span length")
	}
	parent := spans[0]
	child := spans[1]
	assert.Equal(t, child.ParentID, parent.Context().(mocktracer.MockSpanContext).SpanID)
	assert.True(t, parent.Tag("error").(bool))
}
