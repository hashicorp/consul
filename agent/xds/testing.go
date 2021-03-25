package xds

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoy_core_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/mitchellh/go-testing-interface"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/agent/xds/proxysupport"
)

// TestADSStream mocks
// discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer to allow
// testing ADS handler.
type TestADSStream struct {
	stubGrpcServerStream
	sendCh chan *envoy_api_v2.DiscoveryResponse
	recvCh chan *envoy_api_v2.DiscoveryRequest
}

// NewTestADSStream makes a new TestADSStream
func NewTestADSStream(t testing.T, ctx context.Context) *TestADSStream {
	s := &TestADSStream{
		sendCh: make(chan *envoy_api_v2.DiscoveryResponse, 1),
		recvCh: make(chan *envoy_api_v2.DiscoveryRequest, 1),
	}
	s.stubGrpcServerStream.ctx = ctx
	return s
}

// Send implements ADSStream
func (s *TestADSStream) Send(r *envoy_api_v2.DiscoveryResponse) error {
	s.sendCh <- r
	return nil
}

// Recv implements ADSStream
func (s *TestADSStream) Recv() (*envoy_api_v2.DiscoveryRequest, error) {
	r := <-s.recvCh
	if r == nil {
		return nil, io.EOF
	}
	return r, nil
}

// TestEnvoy is a helper to simulate Envoy ADS requests.
type TestEnvoy struct {
	mu sync.Mutex

	ctx    context.Context
	cancel func()

	proxyID string
	token   string

	stream *TestADSStream // SoTW v2
}

// NewTestEnvoy creates a TestEnvoy instance.
func NewTestEnvoy(t testing.T, proxyID, token string) *TestEnvoy {
	ctx, cancel := context.WithCancel(context.Background())
	// If a token is given, attach it to the context in the same way gRPC attaches
	// metadata in calls and stream contexts.
	if token != "" {
		ctx = metadata.NewIncomingContext(ctx,
			metadata.Pairs("x-consul-token", token))
	}
	return &TestEnvoy{
		ctx:    ctx,
		cancel: cancel,

		proxyID: proxyID,
		token:   token,

		stream: NewTestADSStream(t, ctx),
	}
}

func hexString(v uint64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%08x", v)
}

func stringToEnvoyVersion(vs string) (*envoy_type_v3.SemanticVersion, bool) {
	parts := strings.Split(vs, ".")
	if len(parts) != 3 {
		return nil, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, false
	}

	return &envoy_type_v3.SemanticVersion{
		MajorNumber: uint32(major),
		MinorNumber: uint32(minor),
		Patch:       uint32(patch),
	}, true
}

// SendReq sends a request from the test server.
func (e *TestEnvoy) SendReq(t testing.T, typeURL string, version, nonce uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ev, valid := stringToEnvoyVersion(proxysupport.EnvoyVersions[0])
	if !valid {
		t.Fatal("envoy version is not valid: %s", proxysupport.EnvoyVersions[0])
	}

	evV2, err := convertSemanticVersionToV2(ev)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &envoy_api_v2.DiscoveryRequest{
		VersionInfo: hexString(version),
		Node: &envoy_core_v2.Node{
			Id:            e.proxyID,
			Cluster:       e.proxyID,
			UserAgentName: "envoy",
			UserAgentVersionType: &envoy_core_v2.Node_UserAgentBuildVersion{
				UserAgentBuildVersion: &envoy_core_v2.BuildVersion{
					Version: evV2,
				},
			},
		},
		ResponseNonce: hexString(nonce),
		TypeUrl:       typeURL,
	}
	select {
	case e.stream.recvCh <- req:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("send to stream blocked for too long")
	}
}

// Close closes the client and cancels it's request context.
func (e *TestEnvoy) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// unblock the recv chans to simulate recv errors when client disconnects
	if e.stream != nil && e.stream.recvCh != nil {
		close(e.stream.recvCh)
		e.stream = nil
	}
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

type stubGrpcServerStream struct {
	ctx context.Context
	grpc.ServerStream
}

var _ grpc.ServerStream = (*stubGrpcServerStream)(nil)

// SetHeader implements grpc.ServerStream as part of ADSDeltaStream
func (s *stubGrpcServerStream) SetHeader(metadata.MD) error {
	return nil
}

// SendHeader implements grpc.ServerStream as part of ADSDeltaStream
func (s *stubGrpcServerStream) SendHeader(metadata.MD) error {
	return nil
}

// SetTrailer implements grpc.ServerStream as part of ADSDeltaStream
func (s *stubGrpcServerStream) SetTrailer(metadata.MD) {
}

// Context implements grpc.ServerStream as part of ADSDeltaStream
func (s *stubGrpcServerStream) Context() context.Context {
	return s.ctx
}

// SendMsg implements grpc.ServerStream as part of ADSDeltaStream
func (s *stubGrpcServerStream) SendMsg(m interface{}) error {
	return nil
}

// RecvMsg implements grpc.ServerStream as part of ADSDeltaStream
func (s *stubGrpcServerStream) RecvMsg(m interface{}) error {
	return nil
}
