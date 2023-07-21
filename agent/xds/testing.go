package xds

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"

	"github.com/mitchellh/go-testing-interface"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TestADSDeltaStream mocks
// discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer to allow
// testing the ADS handler.
type TestADSDeltaStream struct {
	stubGrpcServerStream
	sendCh chan *envoy_discovery_v3.DeltaDiscoveryResponse
	recvCh chan *envoy_discovery_v3.DeltaDiscoveryRequest

	mu      sync.Mutex
	sendErr error
}

var _ ADSDeltaStream = (*TestADSDeltaStream)(nil)

func NewTestADSDeltaStream(t testing.T, ctx context.Context) *TestADSDeltaStream {
	s := &TestADSDeltaStream{
		sendCh: make(chan *envoy_discovery_v3.DeltaDiscoveryResponse, 1),
		recvCh: make(chan *envoy_discovery_v3.DeltaDiscoveryRequest, 1),
	}
	s.stubGrpcServerStream.ctx = ctx
	return s
}

// Send implements ADSDeltaStream
func (s *TestADSDeltaStream) Send(r *envoy_discovery_v3.DeltaDiscoveryResponse) error {
	s.mu.Lock()
	err := s.sendErr
	s.mu.Unlock()

	if err != nil {
		return err
	}

	s.sendCh <- r
	return nil
}

func (s *TestADSDeltaStream) SetSendErr(err error) {
	s.mu.Lock()
	s.sendErr = err
	s.mu.Unlock()
}

// Recv implements ADSDeltaStream
func (s *TestADSDeltaStream) Recv() (*envoy_discovery_v3.DeltaDiscoveryRequest, error) {
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

	EnvoyVersion string

	deltaStream *TestADSDeltaStream // Incremental v3

	closed bool
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

		deltaStream: NewTestADSDeltaStream(t, ctx),
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

func (e *TestEnvoy) SetSendErr(err error) {
	e.deltaStream.SetSendErr(err)
}

// SendDeltaReq sends a delta request from the test server.
//
// NOTE: the input request is mutated before sending by injecting the node.
func (e *TestEnvoy) SendDeltaReq(
	t testing.T,
	typeURL string,
	req *envoy_discovery_v3.DeltaDiscoveryRequest, // optional
) {
	e.sendDeltaReq(t, typeURL, nil, req)
}

func (e *TestEnvoy) SendDeltaReqACK(
	t testing.T,
	typeURL string,
	nonce uint64,
) {
	e.sendDeltaReq(t, typeURL, &nonce, nil)
}

func (e *TestEnvoy) SendDeltaReqNACK(
	t testing.T,
	typeURL string,
	nonce uint64,
	errorDetail *status.Status,
) {
	e.sendDeltaReq(t, typeURL, &nonce, &envoy_discovery_v3.DeltaDiscoveryRequest{
		ErrorDetail: errorDetail,
	})
}

func (e *TestEnvoy) sendDeltaReq(
	t testing.T,
	typeURL string,
	nonce *uint64,
	req *envoy_discovery_v3.DeltaDiscoveryRequest, // optional
) {
	e.mu.Lock()
	defer e.mu.Unlock()

	stringVersion := e.EnvoyVersion
	if stringVersion == "" {
		stringVersion = xdscommon.EnvoyVersions[0]
	}

	ev, valid := stringToEnvoyVersion(stringVersion)
	if !valid {
		t.Fatal("envoy version is not valid: %s", stringVersion)
	}

	if req == nil {
		req = &envoy_discovery_v3.DeltaDiscoveryRequest{}
	}
	if nonce != nil {
		req.ResponseNonce = hexString(*nonce)
	}
	req.TypeUrl = typeURL

	req.Node = &envoy_core_v3.Node{
		Id:            e.proxyID,
		Cluster:       e.proxyID,
		UserAgentName: "envoy",
		UserAgentVersionType: &envoy_core_v3.Node_UserAgentBuildVersion{
			UserAgentBuildVersion: &envoy_core_v3.BuildVersion{
				Version: ev,
			},
		},
	}

	select {
	case e.deltaStream.recvCh <- req:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("send to delta stream blocked for too long")
	}
}

// Close closes the client and cancels it's request context.
func (e *TestEnvoy) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// unblock the recv chans to simulate recv errors when client disconnects
	if !e.closed && e.deltaStream.recvCh != nil {
		close(e.deltaStream.recvCh)
		e.closed = true
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
