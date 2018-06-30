/*
 *
 * Copyright 2014 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

//go:generate protoc --go_out=plugins=grpc:. codec_perf/perf.proto
//go:generate protoc --go_out=plugins=grpc:. grpc_testing/test.proto

package test

import (
	"bytes"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	anypb "github.com/golang/protobuf/ptypes/any"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
	_ "google.golang.org/grpc/grpclog/glogger"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/internal"
	"google.golang.org/grpc/internal/leakcheck"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	_ "google.golang.org/grpc/resolver/passthrough"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"
	testpb "google.golang.org/grpc/test/grpc_testing"
	"google.golang.org/grpc/testdata"
)

func init() {
	grpc.RegisterChannelz()
}

var (
	// For headers:
	testMetadata = metadata.MD{
		"key1":     []string{"value1"},
		"key2":     []string{"value2"},
		"key3-bin": []string{"binvalue1", string([]byte{1, 2, 3})},
	}
	testMetadata2 = metadata.MD{
		"key1": []string{"value12"},
		"key2": []string{"value22"},
	}
	// For trailers:
	testTrailerMetadata = metadata.MD{
		"tkey1":     []string{"trailerValue1"},
		"tkey2":     []string{"trailerValue2"},
		"tkey3-bin": []string{"trailerbinvalue1", string([]byte{3, 2, 1})},
	}
	testTrailerMetadata2 = metadata.MD{
		"tkey1": []string{"trailerValue12"},
		"tkey2": []string{"trailerValue22"},
	}
	// capital "Key" is illegal in HTTP/2.
	malformedHTTP2Metadata = metadata.MD{
		"Key": []string{"foo"},
	}
	testAppUA     = "myApp1/1.0 myApp2/0.9"
	failAppUA     = "fail-this-RPC"
	detailedError = status.ErrorProto(&spb.Status{
		Code:    int32(codes.DataLoss),
		Message: "error for testing: " + failAppUA,
		Details: []*anypb.Any{{
			TypeUrl: "url",
			Value:   []byte{6, 0, 0, 6, 1, 3},
		}},
	})
)

var raceMode bool // set by race.go in race mode

type testServer struct {
	security           string // indicate the authentication protocol used by this server.
	earlyFail          bool   // whether to error out the execution of a service handler prematurely.
	setAndSendHeader   bool   // whether to call setHeader and sendHeader.
	setHeaderOnly      bool   // whether to only call setHeader, not sendHeader.
	multipleSetTrailer bool   // whether to call setTrailer multiple times.
	unaryCallSleepTime time.Duration
}

func (s *testServer) EmptyCall(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// For testing purpose, returns an error if user-agent is failAppUA.
		// To test that client gets the correct error.
		if ua, ok := md["user-agent"]; !ok || strings.HasPrefix(ua[0], failAppUA) {
			return nil, detailedError
		}
		var str []string
		for _, entry := range md["user-agent"] {
			str = append(str, "ua", entry)
		}
		grpc.SendHeader(ctx, metadata.Pairs(str...))
	}
	return new(testpb.Empty), nil
}

func newPayload(t testpb.PayloadType, size int32) (*testpb.Payload, error) {
	if size < 0 {
		return nil, fmt.Errorf("Requested a response with invalid length %d", size)
	}
	body := make([]byte, size)
	switch t {
	case testpb.PayloadType_COMPRESSABLE:
	case testpb.PayloadType_UNCOMPRESSABLE:
		return nil, fmt.Errorf("PayloadType UNCOMPRESSABLE is not supported")
	default:
		return nil, fmt.Errorf("Unsupported payload type: %d", t)
	}
	return &testpb.Payload{
		Type: t,
		Body: body,
	}, nil
}

func (s *testServer) UnaryCall(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if _, exists := md[":authority"]; !exists {
			return nil, status.Errorf(codes.DataLoss, "expected an :authority metadata: %v", md)
		}
		if s.setAndSendHeader {
			if err := grpc.SetHeader(ctx, md); err != nil {
				return nil, status.Errorf(status.Code(err), "grpc.SetHeader(_, %v) = %v, want <nil>", md, err)
			}
			if err := grpc.SendHeader(ctx, testMetadata2); err != nil {
				return nil, status.Errorf(status.Code(err), "grpc.SendHeader(_, %v) = %v, want <nil>", testMetadata2, err)
			}
		} else if s.setHeaderOnly {
			if err := grpc.SetHeader(ctx, md); err != nil {
				return nil, status.Errorf(status.Code(err), "grpc.SetHeader(_, %v) = %v, want <nil>", md, err)
			}
			if err := grpc.SetHeader(ctx, testMetadata2); err != nil {
				return nil, status.Errorf(status.Code(err), "grpc.SetHeader(_, %v) = %v, want <nil>", testMetadata2, err)
			}
		} else {
			if err := grpc.SendHeader(ctx, md); err != nil {
				return nil, status.Errorf(status.Code(err), "grpc.SendHeader(_, %v) = %v, want <nil>", md, err)
			}
		}
		if err := grpc.SetTrailer(ctx, testTrailerMetadata); err != nil {
			return nil, status.Errorf(status.Code(err), "grpc.SetTrailer(_, %v) = %v, want <nil>", testTrailerMetadata, err)
		}
		if s.multipleSetTrailer {
			if err := grpc.SetTrailer(ctx, testTrailerMetadata2); err != nil {
				return nil, status.Errorf(status.Code(err), "grpc.SetTrailer(_, %v) = %v, want <nil>", testTrailerMetadata2, err)
			}
		}
	}
	pr, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.DataLoss, "failed to get peer from ctx")
	}
	if pr.Addr == net.Addr(nil) {
		return nil, status.Error(codes.DataLoss, "failed to get peer address")
	}
	if s.security != "" {
		// Check Auth info
		var authType, serverName string
		switch info := pr.AuthInfo.(type) {
		case credentials.TLSInfo:
			authType = info.AuthType()
			serverName = info.State.ServerName
		default:
			return nil, status.Error(codes.Unauthenticated, "Unknown AuthInfo type")
		}
		if authType != s.security {
			return nil, status.Errorf(codes.Unauthenticated, "Wrong auth type: got %q, want %q", authType, s.security)
		}
		if serverName != "x.test.youtube.com" {
			return nil, status.Errorf(codes.Unauthenticated, "Unknown server name %q", serverName)
		}
	}
	// Simulate some service delay.
	time.Sleep(s.unaryCallSleepTime)

	payload, err := newPayload(in.GetResponseType(), in.GetResponseSize())
	if err != nil {
		return nil, err
	}

	return &testpb.SimpleResponse{
		Payload: payload,
	}, nil
}

func (s *testServer) StreamingOutputCall(args *testpb.StreamingOutputCallRequest, stream testpb.TestService_StreamingOutputCallServer) error {
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if _, exists := md[":authority"]; !exists {
			return status.Errorf(codes.DataLoss, "expected an :authority metadata: %v", md)
		}
		// For testing purpose, returns an error if user-agent is failAppUA.
		// To test that client gets the correct error.
		if ua, ok := md["user-agent"]; !ok || strings.HasPrefix(ua[0], failAppUA) {
			return status.Error(codes.DataLoss, "error for testing: "+failAppUA)
		}
	}
	cs := args.GetResponseParameters()
	for _, c := range cs {
		if us := c.GetIntervalUs(); us > 0 {
			time.Sleep(time.Duration(us) * time.Microsecond)
		}

		payload, err := newPayload(args.GetResponseType(), c.GetSize())
		if err != nil {
			return err
		}

		if err := stream.Send(&testpb.StreamingOutputCallResponse{
			Payload: payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *testServer) StreamingInputCall(stream testpb.TestService_StreamingInputCallServer) error {
	var sum int
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&testpb.StreamingInputCallResponse{
				AggregatedPayloadSize: int32(sum),
			})
		}
		if err != nil {
			return err
		}
		p := in.GetPayload().GetBody()
		sum += len(p)
		if s.earlyFail {
			return status.Error(codes.NotFound, "not found")
		}
	}
}

func (s *testServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if ok {
		if s.setAndSendHeader {
			if err := stream.SetHeader(md); err != nil {
				return status.Errorf(status.Code(err), "%v.SetHeader(_, %v) = %v, want <nil>", stream, md, err)
			}
			if err := stream.SendHeader(testMetadata2); err != nil {
				return status.Errorf(status.Code(err), "%v.SendHeader(_, %v) = %v, want <nil>", stream, testMetadata2, err)
			}
		} else if s.setHeaderOnly {
			if err := stream.SetHeader(md); err != nil {
				return status.Errorf(status.Code(err), "%v.SetHeader(_, %v) = %v, want <nil>", stream, md, err)
			}
			if err := stream.SetHeader(testMetadata2); err != nil {
				return status.Errorf(status.Code(err), "%v.SetHeader(_, %v) = %v, want <nil>", stream, testMetadata2, err)
			}
		} else {
			if err := stream.SendHeader(md); err != nil {
				return status.Errorf(status.Code(err), "%v.SendHeader(%v) = %v, want %v", stream, md, err, nil)
			}
		}
		stream.SetTrailer(testTrailerMetadata)
		if s.multipleSetTrailer {
			stream.SetTrailer(testTrailerMetadata2)
		}
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			return nil
		}
		if err != nil {
			// to facilitate testSvrWriteStatusEarlyWrite
			if status.Code(err) == codes.ResourceExhausted {
				return status.Errorf(codes.Internal, "fake error for test testSvrWriteStatusEarlyWrite. true error: %s", err.Error())
			}
			return err
		}
		cs := in.GetResponseParameters()
		for _, c := range cs {
			if us := c.GetIntervalUs(); us > 0 {
				time.Sleep(time.Duration(us) * time.Microsecond)
			}

			payload, err := newPayload(in.GetResponseType(), c.GetSize())
			if err != nil {
				return err
			}

			if err := stream.Send(&testpb.StreamingOutputCallResponse{
				Payload: payload,
			}); err != nil {
				// to facilitate testSvrWriteStatusEarlyWrite
				if status.Code(err) == codes.ResourceExhausted {
					return status.Errorf(codes.Internal, "fake error for test testSvrWriteStatusEarlyWrite. true error: %s", err.Error())
				}
				return err
			}
		}
	}
}

func (s *testServer) HalfDuplexCall(stream testpb.TestService_HalfDuplexCallServer) error {
	var msgBuf []*testpb.StreamingOutputCallRequest
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			break
		}
		if err != nil {
			return err
		}
		msgBuf = append(msgBuf, in)
	}
	for _, m := range msgBuf {
		cs := m.GetResponseParameters()
		for _, c := range cs {
			if us := c.GetIntervalUs(); us > 0 {
				time.Sleep(time.Duration(us) * time.Microsecond)
			}

			payload, err := newPayload(m.GetResponseType(), c.GetSize())
			if err != nil {
				return err
			}

			if err := stream.Send(&testpb.StreamingOutputCallResponse{
				Payload: payload,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

type env struct {
	name         string
	network      string // The type of network such as tcp, unix, etc.
	security     string // The security protocol such as TLS, SSH, etc.
	httpHandler  bool   // whether to use the http.Handler ServerTransport; requires TLS
	balancer     string // One of "round_robin", "pick_first", "v1", or "".
	customDialer func(string, string, time.Duration) (net.Conn, error)
}

func (e env) runnable() bool {
	if runtime.GOOS == "windows" && e.network == "unix" {
		return false
	}
	return true
}

func (e env) dialer(addr string, timeout time.Duration) (net.Conn, error) {
	if e.customDialer != nil {
		return e.customDialer(e.network, addr, timeout)
	}
	return net.DialTimeout(e.network, addr, timeout)
}

var (
	tcpClearEnv   = env{name: "tcp-clear-v1-balancer", network: "tcp", balancer: "v1"}
	tcpTLSEnv     = env{name: "tcp-tls-v1-balancer", network: "tcp", security: "tls", balancer: "v1"}
	tcpClearRREnv = env{name: "tcp-clear", network: "tcp", balancer: "round_robin"}
	tcpTLSRREnv   = env{name: "tcp-tls", network: "tcp", security: "tls", balancer: "round_robin"}
	handlerEnv    = env{name: "handler-tls", network: "tcp", security: "tls", httpHandler: true, balancer: "round_robin"}
	noBalancerEnv = env{name: "no-balancer", network: "tcp", security: "tls"}
	allEnv        = []env{tcpClearEnv, tcpTLSEnv, tcpClearRREnv, tcpTLSRREnv, handlerEnv, noBalancerEnv}
)

var onlyEnv = flag.String("only_env", "", "If non-empty, one of 'tcp-clear', 'tcp-tls', 'unix-clear', 'unix-tls', or 'handler-tls' to only run the tests for that environment. Empty means all.")

func listTestEnv() (envs []env) {
	if *onlyEnv != "" {
		for _, e := range allEnv {
			if e.name == *onlyEnv {
				if !e.runnable() {
					panic(fmt.Sprintf("--only_env environment %q does not run on %s", *onlyEnv, runtime.GOOS))
				}
				return []env{e}
			}
		}
		panic(fmt.Sprintf("invalid --only_env value %q", *onlyEnv))
	}
	for _, e := range allEnv {
		if e.runnable() {
			envs = append(envs, e)
		}
	}
	return envs
}

// test is an end-to-end test. It should be created with the newTest
// func, modified as needed, and then started with its startServer method.
// It should be cleaned up with the tearDown method.
type test struct {
	t *testing.T
	e env

	ctx    context.Context // valid for life of test, before tearDown
	cancel context.CancelFunc

	// Configurable knobs, after newTest returns:
	testServer              testpb.TestServiceServer // nil means none
	healthServer            *health.Server           // nil means disabled
	maxStream               uint32
	tapHandle               tap.ServerInHandle
	maxMsgSize              *int
	maxClientReceiveMsgSize *int
	maxClientSendMsgSize    *int
	maxServerReceiveMsgSize *int
	maxServerSendMsgSize    *int
	userAgent               string
	// clientCompression and serverCompression are set to test the deprecated API
	// WithCompressor and WithDecompressor.
	clientCompression bool
	serverCompression bool
	// clientUseCompression is set to test the new compressor registration API UseCompressor.
	clientUseCompression bool
	// clientNopCompression is set to create a compressor whose type is not supported.
	clientNopCompression        bool
	unaryClientInt              grpc.UnaryClientInterceptor
	streamClientInt             grpc.StreamClientInterceptor
	unaryServerInt              grpc.UnaryServerInterceptor
	streamServerInt             grpc.StreamServerInterceptor
	unknownHandler              grpc.StreamHandler
	sc                          <-chan grpc.ServiceConfig
	customCodec                 grpc.Codec
	serverInitialWindowSize     int32
	serverInitialConnWindowSize int32
	clientInitialWindowSize     int32
	clientInitialConnWindowSize int32
	perRPCCreds                 credentials.PerRPCCredentials
	customDialOptions           []grpc.DialOption
	resolverScheme              string
	cliKeepAlive                *keepalive.ClientParameters
	svrKeepAlive                *keepalive.ServerParameters

	// All test dialing is blocking by default. Set this to true if dial
	// should be non-blocking.
	nonBlockingDial bool

	// srv and srvAddr are set once startServer is called.
	srv     *grpc.Server
	srvAddr string

	// srvs and srvAddrs are set once startServers is called.
	srvs     []*grpc.Server
	srvAddrs []string

	cc          *grpc.ClientConn // nil until requested via clientConn
	restoreLogs func()           // nil unless declareLogNoise is used
}

func (te *test) tearDown() {
	if te.cancel != nil {
		te.cancel()
		te.cancel = nil
	}

	if te.cc != nil {
		te.cc.Close()
		te.cc = nil
	}

	if te.restoreLogs != nil {
		te.restoreLogs()
		te.restoreLogs = nil
	}

	if te.srv != nil {
		te.srv.Stop()
	}
	if len(te.srvs) != 0 {
		for _, s := range te.srvs {
			s.Stop()
		}
	}
}

// newTest returns a new test using the provided testing.T and
// environment.  It is returned with default values. Tests should
// modify it before calling its startServer and clientConn methods.
func newTest(t *testing.T, e env) *test {
	te := &test{
		t:         t,
		e:         e,
		maxStream: math.MaxUint32,
	}
	te.ctx, te.cancel = context.WithCancel(context.Background())
	return te
}

func (te *test) listenAndServe(ts testpb.TestServiceServer, listen func(network, address string) (net.Listener, error)) net.Listener {
	te.testServer = ts
	te.t.Logf("Running test in %s environment...", te.e.name)
	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(te.maxStream)}
	if te.maxMsgSize != nil {
		sopts = append(sopts, grpc.MaxMsgSize(*te.maxMsgSize))
	}
	if te.maxServerReceiveMsgSize != nil {
		sopts = append(sopts, grpc.MaxRecvMsgSize(*te.maxServerReceiveMsgSize))
	}
	if te.maxServerSendMsgSize != nil {
		sopts = append(sopts, grpc.MaxSendMsgSize(*te.maxServerSendMsgSize))
	}
	if te.tapHandle != nil {
		sopts = append(sopts, grpc.InTapHandle(te.tapHandle))
	}
	if te.serverCompression {
		sopts = append(sopts,
			grpc.RPCCompressor(grpc.NewGZIPCompressor()),
			grpc.RPCDecompressor(grpc.NewGZIPDecompressor()),
		)
	}
	if te.unaryServerInt != nil {
		sopts = append(sopts, grpc.UnaryInterceptor(te.unaryServerInt))
	}
	if te.streamServerInt != nil {
		sopts = append(sopts, grpc.StreamInterceptor(te.streamServerInt))
	}
	if te.unknownHandler != nil {
		sopts = append(sopts, grpc.UnknownServiceHandler(te.unknownHandler))
	}
	if te.serverInitialWindowSize > 0 {
		sopts = append(sopts, grpc.InitialWindowSize(te.serverInitialWindowSize))
	}
	if te.serverInitialConnWindowSize > 0 {
		sopts = append(sopts, grpc.InitialConnWindowSize(te.serverInitialConnWindowSize))
	}
	la := "localhost:0"
	switch te.e.network {
	case "unix":
		la = "/tmp/testsock" + fmt.Sprintf("%d", time.Now().UnixNano())
		syscall.Unlink(la)
	}
	lis, err := listen(te.e.network, la)
	if err != nil {
		te.t.Fatalf("Failed to listen: %v", err)
	}
	switch te.e.security {
	case "tls":
		creds, err := credentials.NewServerTLSFromFile(testdata.Path("server1.pem"), testdata.Path("server1.key"))
		if err != nil {
			te.t.Fatalf("Failed to generate credentials %v", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	case "clientTimeoutCreds":
		sopts = append(sopts, grpc.Creds(&clientTimeoutCreds{}))
	}
	if te.customCodec != nil {
		sopts = append(sopts, grpc.CustomCodec(te.customCodec))
	}
	if te.svrKeepAlive != nil {
		sopts = append(sopts, grpc.KeepaliveParams(*te.svrKeepAlive))
	}
	s := grpc.NewServer(sopts...)
	te.srv = s
	if te.e.httpHandler {
		internal.TestingUseHandlerImpl(s)
	}
	if te.healthServer != nil {
		healthgrpc.RegisterHealthServer(s, te.healthServer)
	}
	if te.testServer != nil {
		testpb.RegisterTestServiceServer(s, te.testServer)
	}
	addr := la
	switch te.e.network {
	case "unix":
	default:
		_, port, err := net.SplitHostPort(lis.Addr().String())
		if err != nil {
			te.t.Fatalf("Failed to parse listener address: %v", err)
		}
		addr = "localhost:" + port
	}

	go s.Serve(lis)
	te.srvAddr = addr
	return lis
}

func (te *test) startServerWithConnControl(ts testpb.TestServiceServer) *listenerWrapper {
	l := te.listenAndServe(ts, listenWithConnControl)
	return l.(*listenerWrapper)
}

// startServer starts a gRPC server listening. Callers should defer a
// call to te.tearDown to clean up.
func (te *test) startServer(ts testpb.TestServiceServer) {
	te.listenAndServe(ts, net.Listen)
}

type nopCompressor struct {
	grpc.Compressor
}

// NewNopCompressor creates a compressor to test the case that type is not supported.
func NewNopCompressor() grpc.Compressor {
	return &nopCompressor{grpc.NewGZIPCompressor()}
}

func (c *nopCompressor) Type() string {
	return "nop"
}

type nopDecompressor struct {
	grpc.Decompressor
}

// NewNopDecompressor creates a decompressor to test the case that type is not supported.
func NewNopDecompressor() grpc.Decompressor {
	return &nopDecompressor{grpc.NewGZIPDecompressor()}
}

func (d *nopDecompressor) Type() string {
	return "nop"
}

func (te *test) configDial(opts ...grpc.DialOption) ([]grpc.DialOption, string) {
	opts = append(opts, grpc.WithDialer(te.e.dialer), grpc.WithUserAgent(te.userAgent))

	if te.sc != nil {
		opts = append(opts, grpc.WithServiceConfig(te.sc))
	}

	if te.clientCompression {
		opts = append(opts,
			grpc.WithCompressor(grpc.NewGZIPCompressor()),
			grpc.WithDecompressor(grpc.NewGZIPDecompressor()),
		)
	}
	if te.clientUseCompression {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")))
	}
	if te.clientNopCompression {
		opts = append(opts,
			grpc.WithCompressor(NewNopCompressor()),
			grpc.WithDecompressor(NewNopDecompressor()),
		)
	}
	if te.unaryClientInt != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(te.unaryClientInt))
	}
	if te.streamClientInt != nil {
		opts = append(opts, grpc.WithStreamInterceptor(te.streamClientInt))
	}
	if te.maxMsgSize != nil {
		opts = append(opts, grpc.WithMaxMsgSize(*te.maxMsgSize))
	}
	if te.maxClientReceiveMsgSize != nil {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(*te.maxClientReceiveMsgSize)))
	}
	if te.maxClientSendMsgSize != nil {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(*te.maxClientSendMsgSize)))
	}
	switch te.e.security {
	case "tls":
		creds, err := credentials.NewClientTLSFromFile(testdata.Path("ca.pem"), "x.test.youtube.com")
		if err != nil {
			te.t.Fatalf("Failed to load credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	case "clientTimeoutCreds":
		opts = append(opts, grpc.WithTransportCredentials(&clientTimeoutCreds{}))
	default:
		opts = append(opts, grpc.WithInsecure())
	}
	// TODO(bar) switch balancer case "pick_first".
	var scheme string
	if te.resolverScheme == "" {
		scheme = "passthrough:///"
	} else {
		scheme = te.resolverScheme + ":///"
	}
	switch te.e.balancer {
	case "v1":
		opts = append(opts, grpc.WithBalancer(grpc.RoundRobin(nil)))
	case "round_robin":
		opts = append(opts, grpc.WithBalancerName(roundrobin.Name))
	}
	if te.clientInitialWindowSize > 0 {
		opts = append(opts, grpc.WithInitialWindowSize(te.clientInitialWindowSize))
	}
	if te.clientInitialConnWindowSize > 0 {
		opts = append(opts, grpc.WithInitialConnWindowSize(te.clientInitialConnWindowSize))
	}
	if te.perRPCCreds != nil {
		opts = append(opts, grpc.WithPerRPCCredentials(te.perRPCCreds))
	}
	if te.customCodec != nil {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.CallCustomCodec(te.customCodec)))
	}
	if !te.nonBlockingDial && te.srvAddr != "" {
		// Only do a blocking dial if server is up.
		opts = append(opts, grpc.WithBlock())
	}
	if te.srvAddr == "" {
		te.srvAddr = "client.side.only.test"
	}
	if te.cliKeepAlive != nil {
		opts = append(opts, grpc.WithKeepaliveParams(*te.cliKeepAlive))
	}
	opts = append(opts, te.customDialOptions...)
	return opts, scheme
}

func (te *test) clientConnWithConnControl() (*grpc.ClientConn, *dialerWrapper) {
	if te.cc != nil {
		return te.cc, nil
	}
	opts, scheme := te.configDial()
	dw := &dialerWrapper{}
	// overwrite the dialer before
	opts = append(opts, grpc.WithDialer(dw.dialer))
	var err error
	te.cc, err = grpc.Dial(scheme+te.srvAddr, opts...)
	if err != nil {
		te.t.Fatalf("Dial(%q) = %v", scheme+te.srvAddr, err)
	}
	return te.cc, dw
}

func (te *test) clientConn(opts ...grpc.DialOption) *grpc.ClientConn {
	if te.cc != nil {
		return te.cc
	}
	var scheme string
	opts, scheme = te.configDial(opts...)
	var err error
	te.cc, err = grpc.Dial(scheme+te.srvAddr, opts...)
	if err != nil {
		te.t.Fatalf("Dial(%q) = %v", scheme+te.srvAddr, err)
	}
	return te.cc
}

func (te *test) declareLogNoise(phrases ...string) {
	te.restoreLogs = declareLogNoise(te.t, phrases...)
}

func (te *test) withServerTester(fn func(st *serverTester)) {
	c, err := te.e.dialer(te.srvAddr, 10*time.Second)
	if err != nil {
		te.t.Fatal(err)
	}
	defer c.Close()
	if te.e.security == "tls" {
		c = tls.Client(c, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{http2.NextProtoTLS},
		})
	}
	st := newServerTesterFromConn(te.t, c)
	st.greet()
	fn(st)
}

type lazyConn struct {
	net.Conn
	beLazy int32
}

func (l *lazyConn) Write(b []byte) (int, error) {
	if atomic.LoadInt32(&(l.beLazy)) == 1 {
		time.Sleep(time.Second)
	}
	return l.Conn.Write(b)
}

func TestContextDeadlineNotIgnored(t *testing.T) {
	defer leakcheck.Check(t)
	e := noBalancerEnv
	var lc *lazyConn
	e.customDialer = func(network, addr string, timeout time.Duration) (net.Conn, error) {
		conn, err := net.DialTimeout(network, addr, timeout)
		if err != nil {
			return nil, err
		}
		lc = &lazyConn{Conn: conn}
		return lc, nil
	}

	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
	atomic.StoreInt32(&(lc.beLazy), 1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	t1 := time.Now()
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, context.DeadlineExceeded", err)
	}
	if time.Since(t1) > 2*time.Second {
		t.Fatalf("TestService/EmptyCall(_, _) ran over the deadline")
	}
}

func TestTimeoutOnDeadServer(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testTimeoutOnDeadServer(t, e)
	}
}

func testTimeoutOnDeadServer(t *testing.T, e env) {
	te := newTest(t, e)
	te.customDialOptions = []grpc.DialOption{grpc.WithWaitForHandshake()}
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
	te.srv.Stop()

	// Wait for the client to notice the connection is gone.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	state := cc.GetState()
	for ; state == connectivity.Ready && cc.WaitForStateChange(ctx, state); state = cc.GetState() {
	}
	cancel()
	if state == connectivity.Ready {
		t.Fatalf("Timed out waiting for non-ready state")
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	_, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false))
	cancel()
	if e.balancer != "" && status.Code(err) != codes.DeadlineExceeded {
		// If e.balancer == nil, the ac will stop reconnecting because the dialer returns non-temp error,
		// the error will be an internal error.
		t.Fatalf("TestService/EmptyCall(%v, _) = _, %v, want _, error code: %s", ctx, err, codes.DeadlineExceeded)
	}
	awaitNewConnLogOutput()
}

func TestServerGracefulStopIdempotent(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testServerGracefulStopIdempotent(t, e)
	}
}

func testServerGracefulStopIdempotent(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	for i := 0; i < 3; i++ {
		te.srv.GracefulStop()
	}
}

func TestServerGoAway(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testServerGoAway(t, e)
	}
}

func testServerGoAway(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	// Finish an RPC to make sure the connection is good.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
	ch := make(chan struct{})
	go func() {
		te.srv.GracefulStop()
		close(ch)
	}()
	// Loop until the server side GoAway signal is propagated to the client.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); err != nil && status.Code(err) != codes.DeadlineExceeded {
			cancel()
			break
		}
		cancel()
	}
	// A new RPC should fail.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.Unavailable && status.Code(err) != codes.Internal {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s or %s", err, codes.Unavailable, codes.Internal)
	}
	<-ch
	awaitNewConnLogOutput()
}

func TestServerGoAwayPendingRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testServerGoAwayPendingRPC(t, e)
	}
}

func testServerGoAwayPendingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	stream, err := tc.FullDuplexCall(ctx, grpc.FailFast(false))
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	// Finish an RPC to make sure the connection is good.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _, _) = _, %v, want _, <nil>", tc, err)
	}
	ch := make(chan struct{})
	go func() {
		te.srv.GracefulStop()
		close(ch)
	}()
	// Loop until the server side GoAway signal is propagated to the client.
	start := time.Now()
	errored := false
	for time.Since(start) < time.Second {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		_, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false))
		cancel()
		if err != nil {
			errored = true
			break
		}
	}
	if !errored {
		t.Fatalf("GoAway never received by client")
	}
	respParam := []*testpb.ResponseParameters{{Size: 1}}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(100))
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	// The existing RPC should be still good to proceed.
	if err := stream.Send(req); err != nil {
		t.Fatalf("%v.Send(_) = %v, want <nil>", stream, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = _, %v, want _, <nil>", stream, err)
	}
	// The RPC will run until canceled.
	cancel()
	<-ch
	awaitNewConnLogOutput()
}

func TestServerMultipleGoAwayPendingRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testServerMultipleGoAwayPendingRPC(t, e)
	}
}

func testServerMultipleGoAwayPendingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := tc.FullDuplexCall(ctx, grpc.FailFast(false))
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	// Finish an RPC to make sure the connection is good.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _, _) = _, %v, want _, <nil>", tc, err)
	}
	ch1 := make(chan struct{})
	go func() {
		te.srv.GracefulStop()
		close(ch1)
	}()
	ch2 := make(chan struct{})
	go func() {
		te.srv.GracefulStop()
		close(ch2)
	}()
	// Loop until the server side GoAway signal is propagated to the client.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		if _, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); err != nil {
			cancel()
			break
		}
		cancel()
	}
	select {
	case <-ch1:
		t.Fatal("GracefulStop() terminated early")
	case <-ch2:
		t.Fatal("GracefulStop() terminated early")
	default:
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: 1,
		},
	}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(100))
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	// The existing RPC should be still good to proceed.
	if err := stream.Send(req); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = _, %v, want _, <nil>", stream, err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() = %v, want <nil>", stream, err)
	}
	<-ch1
	<-ch2
	cancel()
	awaitNewConnLogOutput()
}

func TestConcurrentClientConnCloseAndServerGoAway(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testConcurrentClientConnCloseAndServerGoAway(t, e)
	}
}

func testConcurrentClientConnCloseAndServerGoAway(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _, _) = _, %v, want _, <nil>", tc, err)
	}
	ch := make(chan struct{})
	// Close ClientConn and Server concurrently.
	go func() {
		te.srv.GracefulStop()
		close(ch)
	}()
	go func() {
		cc.Close()
	}()
	<-ch
}

func TestConcurrentServerStopAndGoAway(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testConcurrentServerStopAndGoAway(t, e)
	}
}

func testConcurrentServerStopAndGoAway(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	stream, err := tc.FullDuplexCall(context.Background(), grpc.FailFast(false))
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	// Finish an RPC to make sure the connection is good.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		t.Fatalf("%v.EmptyCall(_, _, _) = _, %v, want _, <nil>", tc, err)
	}
	ch := make(chan struct{})
	go func() {
		te.srv.GracefulStop()
		close(ch)
	}()
	// Loop until the server side GoAway signal is propagated to the client.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		if _, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); err != nil {
			cancel()
			break
		}
		cancel()
	}
	// Stop the server and close all the connections.
	te.srv.Stop()
	respParam := []*testpb.ResponseParameters{
		{
			Size: 1,
		},
	}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(100))
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	sendStart := time.Now()
	for {
		if err := stream.Send(req); err == io.EOF {
			// stream.Send should eventually send io.EOF
			break
		} else if err != nil {
			// Send should never return a transport-level error.
			t.Fatalf("stream.Send(%v) = %v; want <nil or io.EOF>", req, err)
		}
		if time.Since(sendStart) > 2*time.Second {
			t.Fatalf("stream.Send(_) did not return io.EOF after 2s")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := stream.Recv(); err == nil || err == io.EOF {
		t.Fatalf("%v.Recv() = _, %v, want _, <non-nil, non-EOF>", stream, err)
	}
	<-ch
	awaitNewConnLogOutput()
}

func TestClientConnCloseAfterGoAwayWithActiveStream(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testClientConnCloseAfterGoAwayWithActiveStream(t, e)
	}
}

func testClientConnCloseAfterGoAwayWithActiveStream(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tc.FullDuplexCall(ctx); err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want _, <nil>", tc, err)
	}
	done := make(chan struct{})
	go func() {
		te.srv.GracefulStop()
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cc.Close()
	timeout := time.NewTimer(time.Second)
	select {
	case <-done:
	case <-timeout.C:
		t.Fatalf("Test timed-out.")
	}
}

func TestFailFast(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testFailFast(t, e)
	}
}

func testFailFast(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
	// Stop the server and tear down all the existing connections.
	te.srv.Stop()
	// Loop until the server teardown is propagated to the client.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := tc.EmptyCall(ctx, &testpb.Empty{})
		cancel()
		if status.Code(err) == codes.Unavailable {
			break
		}
		t.Logf("%v.EmptyCall(_, _) = _, %v", tc, err)
		time.Sleep(10 * time.Millisecond)
	}
	// The client keeps reconnecting and ongoing fail-fast RPCs should fail with code.Unavailable.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("TestService/EmptyCall(_, _, _) = _, %v, want _, error code: %s", err, codes.Unavailable)
	}
	if _, err := tc.StreamingInputCall(context.Background()); status.Code(err) != codes.Unavailable {
		t.Fatalf("TestService/StreamingInputCall(_) = _, %v, want _, error code: %s", err, codes.Unavailable)
	}

	awaitNewConnLogOutput()
}

func testServiceConfigSetup(t *testing.T, e env) *test {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"Failed to dial : context canceled; please retry.",
	)
	return te
}

func newBool(b bool) (a *bool) {
	return &b
}

func newInt(b int) (a *int) {
	return &b
}

func newDuration(b time.Duration) (a *time.Duration) {
	a = new(time.Duration)
	*a = b
	return
}

func TestGetMethodConfig(t *testing.T) {
	te := testServiceConfigSetup(t, tcpClearRREnv)
	defer te.tearDown()
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	te.resolverScheme = r.Scheme()
	cc := te.clientConn()
	r.NewAddress([]resolver.Address{{Addr: te.srvAddr}})
	r.NewServiceConfig(`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "EmptyCall"
                }
            ],
            "waitForReady": true,
            "timeout": ".001s"
        },
        {
            "name": [
                {
                    "service": "grpc.testing.TestService"
                }
            ],
            "waitForReady": false
        }
    ]
}`)

	tc := testpb.NewTestServiceClient(cc)

	// Make sure service config has been processed by grpc.
	for {
		if cc.GetMethodConfig("/grpc.testing.TestService/EmptyCall").WaitForReady != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// The following RPCs are expected to become non-fail-fast ones with 1ms deadline.
	var err error
	if _, err = tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}

	r.NewServiceConfig(`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "UnaryCall"
                }
            ],
            "waitForReady": true,
            "timeout": ".001s"
        },
        {
            "name": [
                {
                    "service": "grpc.testing.TestService"
                }
            ],
            "waitForReady": false
        }
    ]
}`)

	// Make sure service config has been processed by grpc.
	for {
		if mc := cc.GetMethodConfig("/grpc.testing.TestService/EmptyCall"); mc.WaitForReady != nil && !*mc.WaitForReady {
			break
		}
		time.Sleep(time.Millisecond)
	}
	// The following RPCs are expected to become fail-fast.
	if _, err = tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.Unavailable)
	}
}

func TestServiceConfigWaitForReady(t *testing.T) {
	te := testServiceConfigSetup(t, tcpClearRREnv)
	defer te.tearDown()
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	// Case1: Client API set failfast to be false, and service config set wait_for_ready to be false, Client API should win, and the rpc will wait until deadline exceeds.
	te.resolverScheme = r.Scheme()
	cc := te.clientConn()
	r.NewAddress([]resolver.Address{{Addr: te.srvAddr}})
	r.NewServiceConfig(`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "EmptyCall"
                },
                {
                    "service": "grpc.testing.TestService",
                    "method": "FullDuplexCall"
                }
            ],
            "waitForReady": false,
            "timeout": ".001s"
        }
    ]
}`)

	tc := testpb.NewTestServiceClient(cc)

	// Make sure service config has been processed by grpc.
	for {
		if cc.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall").WaitForReady != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// The following RPCs are expected to become non-fail-fast ones with 1ms deadline.
	var err error
	if _, err = tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	if _, err := tc.FullDuplexCall(context.Background(), grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}

	// Generate a service config update.
	// Case2:Client API set failfast to be false, and service config set wait_for_ready to be true, and the rpc will wait until deadline exceeds.
	r.NewServiceConfig(`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "EmptyCall"
                },
                {
                    "service": "grpc.testing.TestService",
                    "method": "FullDuplexCall"
                }
            ],
            "waitForReady": true,
            "timeout": ".001s"
        }
    ]
}`)

	// Wait for the new service config to take effect.
	for {
		if mc := cc.GetMethodConfig("/grpc.testing.TestService/EmptyCall"); mc.WaitForReady != nil && *mc.WaitForReady {
			break
		}
		time.Sleep(time.Millisecond)
	}
	// The following RPCs are expected to become non-fail-fast ones with 1ms deadline.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	if _, err := tc.FullDuplexCall(context.Background()); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}
}

func TestServiceConfigTimeout(t *testing.T) {
	te := testServiceConfigSetup(t, tcpClearRREnv)
	defer te.tearDown()
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	// Case1: Client API sets timeout to be 1ns and ServiceConfig sets timeout to be 1hr. Timeout should be 1ns (min of 1ns and 1hr) and the rpc will wait until deadline exceeds.
	te.resolverScheme = r.Scheme()
	cc := te.clientConn()
	r.NewAddress([]resolver.Address{{Addr: te.srvAddr}})
	r.NewServiceConfig(`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "EmptyCall"
                },
                {
                    "service": "grpc.testing.TestService",
                    "method": "FullDuplexCall"
                }
            ],
            "waitForReady": true,
            "timeout": "3600s"
        }
    ]
}`)

	tc := testpb.NewTestServiceClient(cc)

	// Make sure service config has been processed by grpc.
	for {
		if cc.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall").Timeout != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// The following RPCs are expected to become non-fail-fast ones with 1ns deadline.
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	if _, err = tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), time.Nanosecond)
	if _, err = tc.FullDuplexCall(ctx, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}
	cancel()

	// Generate a service config update.
	// Case2: Client API sets timeout to be 1hr and ServiceConfig sets timeout to be 1ns. Timeout should be 1ns (min of 1ns and 1hr) and the rpc will wait until deadline exceeds.
	r.NewServiceConfig(`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "EmptyCall"
                },
                {
                    "service": "grpc.testing.TestService",
                    "method": "FullDuplexCall"
                }
            ],
            "waitForReady": true,
            "timeout": ".000000001s"
        }
    ]
}`)

	// Wait for the new service config to take effect.
	for {
		if mc := cc.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall"); mc.Timeout != nil && *mc.Timeout == time.Nanosecond {
			break
		}
		time.Sleep(time.Millisecond)
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	if _, err = tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	if _, err = tc.FullDuplexCall(ctx, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}
	cancel()
}

func TestServiceConfigMaxMsgSize(t *testing.T) {
	e := tcpClearRREnv
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	// Setting up values and objects shared across all test cases.
	const smallSize = 1
	const largeSize = 1024
	const extraLargeSize = 2048

	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}
	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	extraLargePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, extraLargeSize)
	if err != nil {
		t.Fatal(err)
	}

	scjs := `{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "grpc.testing.TestService",
                    "method": "UnaryCall"
                },
                {
                    "service": "grpc.testing.TestService",
                    "method": "FullDuplexCall"
                }
            ],
            "maxRequestMessageBytes": 2048,
            "maxResponseMessageBytes": 2048
        }
    ]
}`

	// Case1: sc set maxReqSize to 2048 (send), maxRespSize to 2048 (recv).
	te1 := testServiceConfigSetup(t, e)
	defer te1.tearDown()

	te1.resolverScheme = r.Scheme()
	te1.nonBlockingDial = true
	te1.startServer(&testServer{security: e.security})
	cc1 := te1.clientConn()

	r.NewAddress([]resolver.Address{{Addr: te1.srvAddr}})
	r.NewServiceConfig(scjs)
	tc := testpb.NewTestServiceClient(cc1)

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(extraLargeSize),
		Payload:      smallPayload,
	}

	for {
		if cc1.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall").MaxReqSize != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Test for unary RPC recv.
	if _, err = tc.UnaryCall(context.Background(), req, grpc.FailFast(false)); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = extraLargePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(extraLargeSize),
		},
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            smallPayload,
	}
	stream, err := tc.FullDuplexCall(te1.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err = stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err = stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = extraLargePayload
	stream, err = tc.FullDuplexCall(te1.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err = stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}

	// Case2: Client API set maxReqSize to 1024 (send), maxRespSize to 1024 (recv). Sc sets maxReqSize to 2048 (send), maxRespSize to 2048 (recv).
	te2 := testServiceConfigSetup(t, e)
	te2.resolverScheme = r.Scheme()
	te2.nonBlockingDial = true
	te2.maxClientReceiveMsgSize = newInt(1024)
	te2.maxClientSendMsgSize = newInt(1024)

	te2.startServer(&testServer{security: e.security})
	defer te2.tearDown()
	cc2 := te2.clientConn()
	r.NewAddress([]resolver.Address{{Addr: te2.srvAddr}})
	r.NewServiceConfig(scjs)
	tc = testpb.NewTestServiceClient(cc2)

	for {
		if cc2.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall").MaxReqSize != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Test for unary RPC recv.
	req.Payload = smallPayload
	req.ResponseSize = int32(largeSize)

	if _, err = tc.UnaryCall(context.Background(), req, grpc.FailFast(false)); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = largePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	stream, err = tc.FullDuplexCall(te2.ctx)
	respParam[0].Size = int32(largeSize)
	sreq.Payload = smallPayload
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err = stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err = stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = largePayload
	stream, err = tc.FullDuplexCall(te2.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err = stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}

	// Case3: Client API set maxReqSize to 4096 (send), maxRespSize to 4096 (recv). Sc sets maxReqSize to 2048 (send), maxRespSize to 2048 (recv).
	te3 := testServiceConfigSetup(t, e)
	te3.resolverScheme = r.Scheme()
	te3.nonBlockingDial = true
	te3.maxClientReceiveMsgSize = newInt(4096)
	te3.maxClientSendMsgSize = newInt(4096)

	te3.startServer(&testServer{security: e.security})
	defer te3.tearDown()

	cc3 := te3.clientConn()
	r.NewAddress([]resolver.Address{{Addr: te3.srvAddr}})
	r.NewServiceConfig(scjs)
	tc = testpb.NewTestServiceClient(cc3)

	for {
		if cc3.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall").MaxReqSize != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// Test for unary RPC recv.
	req.Payload = smallPayload
	req.ResponseSize = int32(largeSize)

	if _, err = tc.UnaryCall(context.Background(), req, grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want <nil>", err)
	}

	req.ResponseSize = int32(extraLargeSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = largePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want <nil>", err)
	}

	req.Payload = extraLargePayload
	if _, err = tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	stream, err = tc.FullDuplexCall(te3.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam[0].Size = int32(largeSize)
	sreq.Payload = smallPayload

	if err = stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err = stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = _, %v, want <nil>", stream, err)
	}

	respParam[0].Size = int32(extraLargeSize)

	if err = stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err = stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = largePayload
	stream, err = tc.FullDuplexCall(te3.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	sreq.Payload = extraLargePayload
	if err := stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}
}

// Reading from a streaming RPC may fail with context canceled if timeout was
// set by service config (https://github.com/grpc/grpc-go/issues/1818). This
// test makes sure read from streaming RPC doesn't fail in this case.
func TestStreamingRPCWithTimeoutInServiceConfigRecv(t *testing.T) {
	te := testServiceConfigSetup(t, tcpClearRREnv)
	te.startServer(&testServer{security: tcpClearRREnv.security})
	defer te.tearDown()
	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	te.resolverScheme = r.Scheme()
	te.nonBlockingDial = true
	fmt.Println("1")
	cc := te.clientConn()
	fmt.Println("10")
	tc := testpb.NewTestServiceClient(cc)

	r.NewAddress([]resolver.Address{{Addr: te.srvAddr}})
	r.NewServiceConfig(`{
	    "methodConfig": [
	        {
	            "name": [
	                {
	                    "service": "grpc.testing.TestService",
	                    "method": "FullDuplexCall"
	                }
	            ],
	            "waitForReady": true,
	            "timeout": "10s"
	        }
	    ]
	}`)
	// Make sure service config has been processed by grpc.
	for {
		if cc.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall").Timeout != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := tc.FullDuplexCall(ctx, grpc.FailFast(false))
	if err != nil {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want <nil>", err)
	}

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 0)
	if err != nil {
		t.Fatalf("failed to newPayload: %v", err)
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: []*testpb.ResponseParameters{{Size: 0}},
		Payload:            payload,
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("stream.Send(%v) = %v, want <nil>", req, err)
	}
	stream.CloseSend()
	time.Sleep(time.Second)
	// Sleep 1 second before recv to make sure the final status is received
	// before the recv.
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("stream.Recv = _, %v, want _, <nil>", err)
	}
	// Keep reading to drain the stream.
	for {
		if _, err := stream.Recv(); err != nil {
			break
		}
	}
}

func TestMaxMsgSizeClientDefault(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMaxMsgSizeClientDefault(t, e)
	}
}

func testMaxMsgSizeClientDefault(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"Failed to dial : context canceled; please retry.",
	)
	te.startServer(&testServer{security: e.security})

	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const smallSize = 1
	const largeSize = 4 * 1024 * 1024
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(largeSize),
		Payload:      smallPayload,
	}
	// Test for unary RPC recv.
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(largeSize),
		},
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            smallPayload,
	}

	// Test for streaming RPC recv.
	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}
}

func TestMaxMsgSizeClientAPI(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMaxMsgSizeClientAPI(t, e)
	}
}

func testMaxMsgSizeClientAPI(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	// To avoid error on server side.
	te.maxServerSendMsgSize = newInt(5 * 1024 * 1024)
	te.maxClientReceiveMsgSize = newInt(1024)
	te.maxClientSendMsgSize = newInt(1024)
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"Failed to dial : context canceled; please retry.",
	)
	te.startServer(&testServer{security: e.security})

	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const smallSize = 1
	const largeSize = 1024
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}

	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(largeSize),
		Payload:      smallPayload,
	}
	// Test for unary RPC recv.
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = largePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(largeSize),
		},
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            smallPayload,
	}

	// Test for streaming RPC recv.
	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = largePayload
	stream, err = tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}
}

func TestMaxMsgSizeServerAPI(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMaxMsgSizeServerAPI(t, e)
	}
}

func testMaxMsgSizeServerAPI(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.maxServerReceiveMsgSize = newInt(1024)
	te.maxServerSendMsgSize = newInt(1024)
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"Failed to dial : context canceled; please retry.",
	)
	te.startServer(&testServer{security: e.security})

	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const smallSize = 1
	const largeSize = 1024
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}

	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(largeSize),
		Payload:      smallPayload,
	}
	// Test for unary RPC send.
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC recv.
	req.Payload = largePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(largeSize),
		},
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            smallPayload,
	}

	// Test for streaming RPC send.
	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = largePayload
	stream, err = tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}
}

func TestTap(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testTap(t, e)
	}
}

type myTap struct {
	cnt int
}

func (t *myTap) handle(ctx context.Context, info *tap.Info) (context.Context, error) {
	if info != nil {
		if info.FullMethodName == "/grpc.testing.TestService/EmptyCall" {
			t.cnt++
		} else if info.FullMethodName == "/grpc.testing.TestService/UnaryCall" {
			return nil, fmt.Errorf("tap error")
		}
	}
	return ctx, nil
}

func testTap(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	ttap := &myTap{}
	te.tapHandle = ttap.handle
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
	)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
	if ttap.cnt != 1 {
		t.Fatalf("Get the count in ttap %d, want 1", ttap.cnt)
	}

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 31)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: 45,
		Payload:      payload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); status.Code(err) != codes.Unavailable {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, %s", err, codes.Unavailable)
	}
}

func healthCheck(d time.Duration, cc *grpc.ClientConn, serviceName string) (*healthpb.HealthCheckResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	hc := healthgrpc.NewHealthClient(cc)
	req := &healthpb.HealthCheckRequest{
		Service: serviceName,
	}
	return hc.Check(ctx, req)
}

func TestHealthCheckOnSuccess(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testHealthCheckOnSuccess(t, e)
	}
}

func testHealthCheckOnSuccess(t *testing.T, e env) {
	te := newTest(t, e)
	hs := health.NewServer()
	hs.SetServingStatus("grpc.health.v1.Health", 1)
	te.healthServer = hs
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	if _, err := healthCheck(1*time.Second, cc, "grpc.health.v1.Health"); err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
}

func TestHealthCheckOnFailure(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testHealthCheckOnFailure(t, e)
	}
}

func testHealthCheckOnFailure(t *testing.T, e env) {
	defer leakcheck.Check(t)
	te := newTest(t, e)
	te.declareLogNoise(
		"Failed to dial ",
		"grpc: the client connection is closing; please retry",
	)
	hs := health.NewServer()
	hs.SetServingStatus("grpc.health.v1.HealthCheck", 1)
	te.healthServer = hs
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	wantErr := status.Error(codes.DeadlineExceeded, "context deadline exceeded")
	if _, err := healthCheck(0*time.Second, cc, "grpc.health.v1.Health"); !reflect.DeepEqual(err, wantErr) {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, error code %s", err, codes.DeadlineExceeded)
	}
	awaitNewConnLogOutput()
}

func TestHealthCheckOff(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		// TODO(bradfitz): Temporarily skip this env due to #619.
		if e.name == "handler-tls" {
			continue
		}
		testHealthCheckOff(t, e)
	}
}

func testHealthCheckOff(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	want := status.Error(codes.Unimplemented, "unknown service grpc.health.v1.Health")
	if _, err := healthCheck(1*time.Second, te.clientConn(), ""); !reflect.DeepEqual(err, want) {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, %v", err, want)
	}
}

func TestUnknownHandler(t *testing.T) {
	defer leakcheck.Check(t)
	// An example unknownHandler that returns a different code and a different method, making sure that we do not
	// expose what methods are implemented to a client that is not authenticated.
	unknownHandler := func(srv interface{}, stream grpc.ServerStream) error {
		return status.Error(codes.Unauthenticated, "user unauthenticated")
	}
	for _, e := range listTestEnv() {
		// TODO(bradfitz): Temporarily skip this env due to #619.
		if e.name == "handler-tls" {
			continue
		}
		testUnknownHandler(t, e, unknownHandler)
	}
}

func testUnknownHandler(t *testing.T, e env, unknownHandler grpc.StreamHandler) {
	te := newTest(t, e)
	te.unknownHandler = unknownHandler
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	want := status.Error(codes.Unauthenticated, "user unauthenticated")
	if _, err := healthCheck(1*time.Second, te.clientConn(), ""); !reflect.DeepEqual(err, want) {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, %v", err, want)
	}
}

func TestHealthCheckServingStatus(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testHealthCheckServingStatus(t, e)
	}
}

func testHealthCheckServingStatus(t *testing.T, e env) {
	te := newTest(t, e)
	hs := health.NewServer()
	te.healthServer = hs
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	out, err := healthCheck(1*time.Second, cc, "")
	if err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
	if out.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("Got the serving status %v, want SERVING", out.Status)
	}
	wantErr := status.Error(codes.NotFound, "unknown service")
	if _, err := healthCheck(1*time.Second, cc, "grpc.health.v1.Health"); !reflect.DeepEqual(err, wantErr) {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, error code %s", err, codes.NotFound)
	}
	hs.SetServingStatus("grpc.health.v1.Health", healthpb.HealthCheckResponse_SERVING)
	out, err = healthCheck(1*time.Second, cc, "grpc.health.v1.Health")
	if err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
	if out.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("Got the serving status %v, want SERVING", out.Status)
	}
	hs.SetServingStatus("grpc.health.v1.Health", healthpb.HealthCheckResponse_NOT_SERVING)
	out, err = healthCheck(1*time.Second, cc, "grpc.health.v1.Health")
	if err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
	if out.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("Got the serving status %v, want NOT_SERVING", out.Status)
	}

}

func TestEmptyUnaryWithUserAgent(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testEmptyUnaryWithUserAgent(t, e)
	}
}

func testEmptyUnaryWithUserAgent(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	var header metadata.MD
	reply, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.Header(&header))
	if err != nil || !proto.Equal(&testpb.Empty{}, reply) {
		t.Fatalf("TestService/EmptyCall(_, _) = %v, %v, want %v, <nil>", reply, err, &testpb.Empty{})
	}
	if v, ok := header["ua"]; !ok || !strings.HasPrefix(v[0], testAppUA) {
		t.Fatalf("header[\"ua\"] = %q, %t, want string with prefix %q, true", v, ok, testAppUA)
	}

	te.srv.Stop()
}

func TestFailedEmptyUnary(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			// This test covers status details, but
			// Grpc-Status-Details-Bin is not support in handler_server.
			continue
		}
		testFailedEmptyUnary(t, e)
	}
}

func testFailedEmptyUnary(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = failAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	wantErr := detailedError
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); !reflect.DeepEqual(err, wantErr) {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %v", err, wantErr)
	}
}

func TestLargeUnary(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testLargeUnary(t, e)
	}
}

func testLargeUnary(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const argSize = 271828
	const respSize = 314159

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	reply, err := tc.UnaryCall(context.Background(), req)
	if err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	pt := reply.GetPayload().GetType()
	ps := len(reply.GetPayload().GetBody())
	if pt != testpb.PayloadType_COMPRESSABLE || ps != respSize {
		t.Fatalf("Got the reply with type %d len %d; want %d, %d", pt, ps, testpb.PayloadType_COMPRESSABLE, respSize)
	}
}

// Test backward-compatibility API for setting msg size limit.
func TestExceedMsgLimit(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testExceedMsgLimit(t, e)
	}
}

func testExceedMsgLimit(t *testing.T, e env) {
	te := newTest(t, e)
	te.maxMsgSize = newInt(1024)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	argSize := int32(*te.maxMsgSize + 1)
	const smallSize = 1

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}

	// Test on server side for unary RPC.
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: smallSize,
		Payload:      payload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}
	// Test on client side for unary RPC.
	req.ResponseSize = int32(*te.maxMsgSize) + 1
	req.Payload = smallPayload
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test on server side for streaming RPC.
	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: 1,
		},
	}

	spayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(*te.maxMsgSize+1))
	if err != nil {
		t.Fatal(err)
	}

	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            spayload,
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test on client side for streaming RPC.
	stream, err = tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam[0].Size = int32(*te.maxMsgSize) + 1
	sreq.Payload = smallPayload
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

}

func TestPeerClientSide(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPeerClientSide(t, e)
	}
}

func testPeerClientSide(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())
	peer := new(peer.Peer)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.Peer(peer), grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
	pa := peer.Addr.String()
	if e.network == "unix" {
		if pa != te.srvAddr {
			t.Fatalf("peer.Addr = %v, want %v", pa, te.srvAddr)
		}
		return
	}
	_, pp, err := net.SplitHostPort(pa)
	if err != nil {
		t.Fatalf("Failed to parse address from peer.")
	}
	_, sp, err := net.SplitHostPort(te.srvAddr)
	if err != nil {
		t.Fatalf("Failed to parse address of test server.")
	}
	if pp != sp {
		t.Fatalf("peer.Addr = localhost:%v, want localhost:%v", pp, sp)
	}
}

// TestPeerNegative tests that if call fails setting peer
// doesn't cause a segmentation fault.
// issue#1141 https://github.com/grpc/grpc-go/issues/1141
func TestPeerNegative(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPeerNegative(t, e)
	}
}

func testPeerNegative(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	peer := new(peer.Peer)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tc.EmptyCall(ctx, &testpb.Empty{}, grpc.Peer(peer))
}

func TestPeerFailedRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPeerFailedRPC(t, e)
	}
}

func testPeerFailedRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.maxServerReceiveMsgSize = newInt(1 * 1024)
	te.startServer(&testServer{security: e.security})

	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	// first make a successful request to the server
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}

	// make a second request that will be rejected by the server
	const largeSize = 5 * 1024
	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		Payload:      largePayload,
	}

	peer := new(peer.Peer)
	if _, err := tc.UnaryCall(context.Background(), req, grpc.Peer(peer)); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	} else {
		pa := peer.Addr.String()
		if e.network == "unix" {
			if pa != te.srvAddr {
				t.Fatalf("peer.Addr = %v, want %v", pa, te.srvAddr)
			}
			return
		}
		_, pp, err := net.SplitHostPort(pa)
		if err != nil {
			t.Fatalf("Failed to parse address from peer.")
		}
		_, sp, err := net.SplitHostPort(te.srvAddr)
		if err != nil {
			t.Fatalf("Failed to parse address of test server.")
		}
		if pp != sp {
			t.Fatalf("peer.Addr = localhost:%v, want localhost:%v", pp, sp)
		}
	}
}

func TestMetadataUnaryRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMetadataUnaryRPC(t, e)
	}
}

func testMetadataUnaryRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	var header, trailer metadata.MD
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	if _, err := tc.UnaryCall(ctx, req, grpc.Header(&header), grpc.Trailer(&trailer)); err != nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <nil>", ctx, err)
	}
	// Ignore optional response headers that Servers may set:
	if header != nil {
		delete(header, "trailer") // RFC 2616 says server SHOULD (but optional) declare trailers
		delete(header, "date")    // the Date header is also optional
		delete(header, "user-agent")
		delete(header, "content-type")
	}
	if !reflect.DeepEqual(header, testMetadata) {
		t.Fatalf("Received header metadata %v, want %v", header, testMetadata)
	}
	if !reflect.DeepEqual(trailer, testTrailerMetadata) {
		t.Fatalf("Received trailer metadata %v, want %v", trailer, testTrailerMetadata)
	}
}

func TestMultipleSetTrailerUnaryRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMultipleSetTrailerUnaryRPC(t, e)
	}
}

func testMultipleSetTrailerUnaryRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, multipleSetTrailer: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = 1
	)
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	var trailer metadata.MD
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	if _, err := tc.UnaryCall(ctx, req, grpc.Trailer(&trailer), grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <nil>", ctx, err)
	}
	expectedTrailer := metadata.Join(testTrailerMetadata, testTrailerMetadata2)
	if !reflect.DeepEqual(trailer, expectedTrailer) {
		t.Fatalf("Received trailer metadata %v, want %v", trailer, expectedTrailer)
	}
}

func TestMultipleSetTrailerStreamingRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMultipleSetTrailerStreamingRPC(t, e)
	}
}

func testMultipleSetTrailerStreamingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, multipleSetTrailer: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	stream, err := tc.FullDuplexCall(ctx, grpc.FailFast(false))
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v failed to complele the FullDuplexCall: %v", stream, err)
	}

	trailer := stream.Trailer()
	expectedTrailer := metadata.Join(testTrailerMetadata, testTrailerMetadata2)
	if !reflect.DeepEqual(trailer, expectedTrailer) {
		t.Fatalf("Received trailer metadata %v, want %v", trailer, expectedTrailer)
	}
}

func TestSetAndSendHeaderUnaryRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testSetAndSendHeaderUnaryRPC(t, e)
	}
}

// To test header metadata is sent on SendHeader().
func testSetAndSendHeaderUnaryRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, setAndSendHeader: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = 1
	)
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	var header metadata.MD
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	if _, err := tc.UnaryCall(ctx, req, grpc.Header(&header), grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <nil>", ctx, err)
	}
	delete(header, "user-agent")
	delete(header, "content-type")
	expectedHeader := metadata.Join(testMetadata, testMetadata2)
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("Received header metadata %v, want %v", header, expectedHeader)
	}
}

func TestMultipleSetHeaderUnaryRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testMultipleSetHeaderUnaryRPC(t, e)
	}
}

// To test header metadata is sent when sending response.
func testMultipleSetHeaderUnaryRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, setHeaderOnly: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = 1
	)
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}

	var header metadata.MD
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	if _, err := tc.UnaryCall(ctx, req, grpc.Header(&header), grpc.FailFast(false)); err != nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <nil>", ctx, err)
	}
	delete(header, "user-agent")
	delete(header, "content-type")
	expectedHeader := metadata.Join(testMetadata, testMetadata2)
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("Received header metadata %v, want %v", header, expectedHeader)
	}
}

func TestMultipleSetHeaderUnaryRPCError(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testMultipleSetHeaderUnaryRPCError(t, e)
	}
}

// To test header metadata is sent when sending status.
func testMultipleSetHeaderUnaryRPCError(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, setHeaderOnly: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = -1 // Invalid respSize to make RPC fail.
	)
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	var header metadata.MD
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	if _, err := tc.UnaryCall(ctx, req, grpc.Header(&header), grpc.FailFast(false)); err == nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <non-nil>", ctx, err)
	}
	delete(header, "user-agent")
	delete(header, "content-type")
	expectedHeader := metadata.Join(testMetadata, testMetadata2)
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("Received header metadata %v, want %v", header, expectedHeader)
	}
}

func TestSetAndSendHeaderStreamingRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testSetAndSendHeaderStreamingRPC(t, e)
	}
}

// To test header metadata is sent on SendHeader().
func testSetAndSendHeaderStreamingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, setAndSendHeader: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = 1
	)
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v failed to complele the FullDuplexCall: %v", stream, err)
	}

	header, err := stream.Header()
	if err != nil {
		t.Fatalf("%v.Header() = _, %v, want _, <nil>", stream, err)
	}
	delete(header, "user-agent")
	delete(header, "content-type")
	expectedHeader := metadata.Join(testMetadata, testMetadata2)
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("Received header metadata %v, want %v", header, expectedHeader)
	}
}

func TestMultipleSetHeaderStreamingRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testMultipleSetHeaderStreamingRPC(t, e)
	}
}

// To test header metadata is sent when sending response.
func testMultipleSetHeaderStreamingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, setHeaderOnly: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = 1
	)
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.StreamingOutputCallRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: []*testpb.ResponseParameters{
			{Size: respSize},
		},
		Payload: payload,
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v failed to complele the FullDuplexCall: %v", stream, err)
	}

	header, err := stream.Header()
	if err != nil {
		t.Fatalf("%v.Header() = _, %v, want _, <nil>", stream, err)
	}
	delete(header, "user-agent")
	delete(header, "content-type")
	expectedHeader := metadata.Join(testMetadata, testMetadata2)
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("Received header metadata %v, want %v", header, expectedHeader)
	}

}

func TestMultipleSetHeaderStreamingRPCError(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testMultipleSetHeaderStreamingRPCError(t, e)
	}
}

// To test header metadata is sent when sending status.
func testMultipleSetHeaderStreamingRPCError(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, setHeaderOnly: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const (
		argSize  = 1
		respSize = -1
	)
	ctx := metadata.NewOutgoingContext(context.Background(), testMetadata)
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.StreamingOutputCallRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: []*testpb.ResponseParameters{
			{Size: respSize},
		},
		Payload: payload,
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatalf("%v.Recv() = %v, want <non-nil>", stream, err)
	}

	header, err := stream.Header()
	if err != nil {
		t.Fatalf("%v.Header() = _, %v, want _, <nil>", stream, err)
	}
	delete(header, "user-agent")
	delete(header, "content-type")
	expectedHeader := metadata.Join(testMetadata, testMetadata2)
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("Received header metadata %v, want %v", header, expectedHeader)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
}

// TestMalformedHTTP2Metedata verfies the returned error when the client
// sends an illegal metadata.
func TestMalformedHTTP2Metadata(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			// Failed with "server stops accepting new RPCs".
			// Server stops accepting new RPCs when the client sends an illegal http2 header.
			continue
		}
		testMalformedHTTP2Metadata(t, e)
	}
}

func testMalformedHTTP2Metadata(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 2718)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: 314,
		Payload:      payload,
	}
	ctx := metadata.NewOutgoingContext(context.Background(), malformedHTTP2Metadata)
	if _, err := tc.UnaryCall(ctx, req); status.Code(err) != codes.Internal {
		t.Fatalf("TestService.UnaryCall(%v, _) = _, %v; want _, %s", ctx, err, codes.Internal)
	}
}

func TestRetry(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			// Fails with RST_STREAM / FLOW_CONTROL_ERROR
			continue
		}
		testRetry(t, e)
	}
}

// This test make sure RPCs are retried times when they receive a RST_STREAM
// with the REFUSED_STREAM error code, which the InTapHandle provokes.
func testRetry(t *testing.T, e env) {
	te := newTest(t, e)
	attempts := 0
	successAttempt := 2
	te.tapHandle = func(ctx context.Context, _ *tap.Info) (context.Context, error) {
		attempts++
		if attempts < successAttempt {
			return nil, errors.New("not now")
		}
		return ctx, nil
	}
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tsc := testpb.NewTestServiceClient(cc)
	testCases := []struct {
		successAttempt int
		failFast       bool
		errCode        codes.Code
	}{{
		successAttempt: 1,
	}, {
		successAttempt: 2,
	}, {
		successAttempt: 3,
		errCode:        codes.Unavailable,
	}, {
		successAttempt: 1,
		failFast:       true,
	}, {
		successAttempt: 2,
		failFast:       true,
		errCode:        codes.Unavailable, // We won't retry on fail fast.
	}}
	for _, tc := range testCases {
		attempts = 0
		successAttempt = tc.successAttempt

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := tsc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(tc.failFast))
		cancel()
		if status.Code(err) != tc.errCode {
			t.Errorf("%+v: tsc.EmptyCall(_, _) = _, %v, want _, Code=%v", tc, err, tc.errCode)
		}
	}
}

func TestCancel(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testCancel(t, e)
	}
}

func testCancel(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("grpc: the client connection is closing; please retry")
	te.startServer(&testServer{security: e.security, unaryCallSleepTime: time.Second})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(1*time.Millisecond, cancel)
	if r, err := tc.UnaryCall(ctx, req); status.Code(err) != codes.Canceled {
		t.Fatalf("TestService/UnaryCall(_, _) = %v, %v; want _, error code: %s", r, err, codes.Canceled)
	}
	awaitNewConnLogOutput()
}

func TestCancelNoIO(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testCancelNoIO(t, e)
	}
}

func testCancelNoIO(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("http2Client.notifyError got notified that the client transport was broken")
	te.maxStream = 1 // Only allows 1 live stream per server transport.
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	// Start one blocked RPC for which we'll never send streaming
	// input. This will consume the 1 maximum concurrent streams,
	// causing future RPCs to hang.
	ctx, cancelFirst := context.WithCancel(context.Background())
	_, err := tc.StreamingInputCall(ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, <nil>", tc, err)
	}

	// Loop until the ClientConn receives the initial settings
	// frame from the server, notifying it about the maximum
	// concurrent streams. We know when it's received it because
	// an RPC will fail with codes.DeadlineExceeded instead of
	// succeeding.
	// TODO(bradfitz): add internal test hook for this (Issue 534)
	for {
		ctx, cancelSecond := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_, err := tc.StreamingInputCall(ctx)
		cancelSecond()
		if err == nil {
			continue
		}
		if status.Code(err) == codes.DeadlineExceeded {
			break
		}
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, %s", tc, err, codes.DeadlineExceeded)
	}
	// If there are any RPCs in flight before the client receives
	// the max streams setting, let them be expired.
	// TODO(bradfitz): add internal test hook for this (Issue 534)
	time.Sleep(50 * time.Millisecond)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancelFirst()
	}()

	// This should be blocked until the 1st is canceled, then succeed.
	ctx, cancelThird := context.WithTimeout(context.Background(), 500*time.Millisecond)
	if _, err := tc.StreamingInputCall(ctx); err != nil {
		t.Errorf("%v.StreamingInputCall(_) = _, %v, want _, <nil>", tc, err)
	}
	cancelThird()
}

// The following tests the gRPC streaming RPC implementations.
// TODO(zhaoq): Have better coverage on error cases.
var (
	reqSizes  = []int{27182, 8, 1828, 45904}
	respSizes = []int{31415, 9, 2653, 58979}
)

func TestNoService(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testNoService(t, e)
	}
}

func testNoService(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(nil)
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	stream, err := tc.FullDuplexCall(te.ctx, grpc.FailFast(false))
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.Unimplemented {
		t.Fatalf("stream.Recv() = _, %v, want _, error code %s", err, codes.Unimplemented)
	}
}

func TestPingPong(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPingPong(t, e)
	}
}

func testPingPong(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	var index int
	for index < len(reqSizes) {
		respParam := []*testpb.ResponseParameters{
			{
				Size: int32(respSizes[index]),
			},
		}

		payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(reqSizes[index]))
		if err != nil {
			t.Fatal(err)
		}

		req := &testpb.StreamingOutputCallRequest{
			ResponseType:       testpb.PayloadType_COMPRESSABLE,
			ResponseParameters: respParam,
			Payload:            payload,
		}
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
		}
		reply, err := stream.Recv()
		if err != nil {
			t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
		}
		pt := reply.GetPayload().GetType()
		if pt != testpb.PayloadType_COMPRESSABLE {
			t.Fatalf("Got the reply of type %d, want %d", pt, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != int(respSizes[index]) {
			t.Fatalf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v failed to complele the ping pong test: %v", stream, err)
	}
}

func TestMetadataStreamingRPC(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testMetadataStreamingRPC(t, e)
	}
}

func testMetadataStreamingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	ctx := metadata.NewOutgoingContext(te.ctx, testMetadata)
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	go func() {
		headerMD, err := stream.Header()
		if e.security == "tls" {
			delete(headerMD, "transport_security_type")
		}
		delete(headerMD, "trailer") // ignore if present
		delete(headerMD, "user-agent")
		delete(headerMD, "content-type")
		if err != nil || !reflect.DeepEqual(testMetadata, headerMD) {
			t.Errorf("#1 %v.Header() = %v, %v, want %v, <nil>", stream, headerMD, err, testMetadata)
		}
		// test the cached value.
		headerMD, err = stream.Header()
		delete(headerMD, "trailer") // ignore if present
		delete(headerMD, "user-agent")
		delete(headerMD, "content-type")
		if err != nil || !reflect.DeepEqual(testMetadata, headerMD) {
			t.Errorf("#2 %v.Header() = %v, %v, want %v, <nil>", stream, headerMD, err, testMetadata)
		}
		err = func() error {
			for index := 0; index < len(reqSizes); index++ {
				respParam := []*testpb.ResponseParameters{
					{
						Size: int32(respSizes[index]),
					},
				}

				payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(reqSizes[index]))
				if err != nil {
					return err
				}

				req := &testpb.StreamingOutputCallRequest{
					ResponseType:       testpb.PayloadType_COMPRESSABLE,
					ResponseParameters: respParam,
					Payload:            payload,
				}
				if err := stream.Send(req); err != nil {
					return fmt.Errorf("%v.Send(%v) = %v, want <nil>", stream, req, err)
				}
			}
			return nil
		}()
		// Tell the server we're done sending args.
		stream.CloseSend()
		if err != nil {
			t.Error(err)
		}
	}()
	for {
		if _, err := stream.Recv(); err != nil {
			break
		}
	}
	trailerMD := stream.Trailer()
	if !reflect.DeepEqual(testTrailerMetadata, trailerMD) {
		t.Fatalf("%v.Trailer() = %v, want %v", stream, trailerMD, testTrailerMetadata)
	}
}

func TestServerStreaming(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testServerStreaming(t, e)
	}
}

func testServerStreaming(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: int32(s),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
	}
	stream, err := tc.StreamingOutputCall(context.Background(), req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
	}
	var rpcStatus error
	var respCnt int
	var index int
	for {
		reply, err := stream.Recv()
		if err != nil {
			rpcStatus = err
			break
		}
		pt := reply.GetPayload().GetType()
		if pt != testpb.PayloadType_COMPRESSABLE {
			t.Fatalf("Got the reply of type %d, want %d", pt, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != int(respSizes[index]) {
			t.Fatalf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
		respCnt++
	}
	if rpcStatus != io.EOF {
		t.Fatalf("Failed to finish the server streaming rpc: %v, want <EOF>", rpcStatus)
	}
	if respCnt != len(respSizes) {
		t.Fatalf("Got %d reply, want %d", len(respSizes), respCnt)
	}
}

func TestFailedServerStreaming(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testFailedServerStreaming(t, e)
	}
}

func testFailedServerStreaming(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = failAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: int32(s),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
	}
	ctx := metadata.NewOutgoingContext(te.ctx, testMetadata)
	stream, err := tc.StreamingOutputCall(ctx, req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
	}
	wantErr := status.Error(codes.DataLoss, "error for testing: "+failAppUA)
	if _, err := stream.Recv(); !reflect.DeepEqual(err, wantErr) {
		t.Fatalf("%v.Recv() = _, %v, want _, %v", stream, err, wantErr)
	}
}

// concurrentSendServer is a TestServiceServer whose
// StreamingOutputCall makes ten serial Send calls, sending payloads
// "0".."9", inclusive.  TestServerStreamingConcurrent verifies they
// were received in the correct order, and that there were no races.
//
// All other TestServiceServer methods crash if called.
type concurrentSendServer struct {
	testpb.TestServiceServer
}

func (s concurrentSendServer) StreamingOutputCall(args *testpb.StreamingOutputCallRequest, stream testpb.TestService_StreamingOutputCallServer) error {
	for i := 0; i < 10; i++ {
		stream.Send(&testpb.StreamingOutputCallResponse{
			Payload: &testpb.Payload{
				Body: []byte{'0' + uint8(i)},
			},
		})
	}
	return nil
}

// Tests doing a bunch of concurrent streaming output calls.
func TestServerStreamingConcurrent(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testServerStreamingConcurrent(t, e)
	}
}

func testServerStreamingConcurrent(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(concurrentSendServer{})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	doStreamingCall := func() {
		req := &testpb.StreamingOutputCallRequest{}
		stream, err := tc.StreamingOutputCall(context.Background(), req)
		if err != nil {
			t.Errorf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
			return
		}
		var ngot int
		var buf bytes.Buffer
		for {
			reply, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			ngot++
			if buf.Len() > 0 {
				buf.WriteByte(',')
			}
			buf.Write(reply.GetPayload().GetBody())
		}
		if want := 10; ngot != want {
			t.Errorf("Got %d replies, want %d", ngot, want)
		}
		if got, want := buf.String(), "0,1,2,3,4,5,6,7,8,9"; got != want {
			t.Errorf("Got replies %q; want %q", got, want)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doStreamingCall()
		}()
	}
	wg.Wait()

}

func generatePayloadSizes() [][]int {
	reqSizes := [][]int{
		{27182, 8, 1828, 45904},
	}

	num8KPayloads := 1024
	eightKPayloads := []int{}
	for i := 0; i < num8KPayloads; i++ {
		eightKPayloads = append(eightKPayloads, (1 << 13))
	}
	reqSizes = append(reqSizes, eightKPayloads)

	num2MPayloads := 8
	twoMPayloads := []int{}
	for i := 0; i < num2MPayloads; i++ {
		twoMPayloads = append(twoMPayloads, (1 << 21))
	}
	reqSizes = append(reqSizes, twoMPayloads)

	return reqSizes
}

func TestClientStreaming(t *testing.T) {
	defer leakcheck.Check(t)
	for _, s := range generatePayloadSizes() {
		for _, e := range listTestEnv() {
			testClientStreaming(t, e, s)
		}
	}
}

func testClientStreaming(t *testing.T, e env, sizes []int) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	ctx, cancel := context.WithTimeout(te.ctx, time.Second*30)
	defer cancel()
	stream, err := tc.StreamingInputCall(ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want <nil>", tc, err)
	}

	var sum int
	for _, s := range sizes {
		payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(s))
		if err != nil {
			t.Fatal(err)
		}

		req := &testpb.StreamingInputCallRequest{
			Payload: payload,
		}
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(_) = %v, want <nil>", stream, err)
		}
		sum += s
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	if reply.GetAggregatedPayloadSize() != int32(sum) {
		t.Fatalf("%v.CloseAndRecv().GetAggregatePayloadSize() = %v; want %v", stream, reply.GetAggregatedPayloadSize(), sum)
	}
}

func TestClientStreamingError(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.name == "handler-tls" {
			continue
		}
		testClientStreamingError(t, e)
	}
}

func testClientStreamingError(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, earlyFail: true})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	stream, err := tc.StreamingInputCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want <nil>", tc, err)
	}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.StreamingInputCallRequest{
		Payload: payload,
	}
	// The 1st request should go through.
	if err := stream.Send(req); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
	}
	for {
		if err := stream.Send(req); err != io.EOF {
			continue
		}
		if _, err := stream.CloseAndRecv(); status.Code(err) != codes.NotFound {
			t.Fatalf("%v.CloseAndRecv() = %v, want error %s", stream, err, codes.NotFound)
		}
		break
	}
}

func TestExceedMaxStreamsLimit(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testExceedMaxStreamsLimit(t, e)
	}
}

func testExceedMaxStreamsLimit(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise(
		"http2Client.notifyError got notified that the client transport was broken",
		"Conn.resetTransport failed to create client transport",
		"grpc: the connection is closing",
	)
	te.maxStream = 1 // Only allows 1 live stream per server transport.
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	_, err := tc.StreamingInputCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, <nil>", tc, err)
	}
	// Loop until receiving the new max stream setting from the server.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := tc.StreamingInputCall(ctx)
		if err == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if status.Code(err) == codes.DeadlineExceeded {
			break
		}
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, %s", tc, err, codes.DeadlineExceeded)
	}
}

func TestStreamsQuotaRecovery(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testStreamsQuotaRecovery(t, e)
	}
}

func testStreamsQuotaRecovery(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise(
		"http2Client.notifyError got notified that the client transport was broken",
		"Conn.resetTransport failed to create client transport",
		"grpc: the connection is closing",
	)
	te.maxStream = 1 // Allows 1 live stream.
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tc.StreamingInputCall(ctx); err != nil {
		t.Fatalf("tc.StreamingInputCall(_) = _, %v, want _, <nil>", err)
	}
	// Loop until the new max stream setting is effective.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_, err := tc.StreamingInputCall(ctx)
		cancel()
		if err == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if status.Code(err) == codes.DeadlineExceeded {
			break
		}
		t.Fatalf("tc.StreamingInputCall(_) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 314)
			if err != nil {
				t.Error(err)
				return
			}
			req := &testpb.SimpleRequest{
				ResponseType: testpb.PayloadType_COMPRESSABLE,
				ResponseSize: 1592,
				Payload:      payload,
			}
			// No rpc should go through due to the max streams limit.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			if _, err := tc.UnaryCall(ctx, req, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
				t.Errorf("tc.UnaryCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
			}
		}()
	}
	wg.Wait()

	cancel()
	// A new stream should be allowed after canceling the first one.
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := tc.StreamingInputCall(ctx); err != nil {
		t.Fatalf("tc.StreamingInputCall(_) = _, %v, want _, %v", err, nil)
	}
}

func TestCompressServerHasNoSupport(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testCompressServerHasNoSupport(t, e)
	}
}

func testCompressServerHasNoSupport(t *testing.T, e env) {
	te := newTest(t, e)
	te.serverCompression = false
	te.clientCompression = false
	te.clientNopCompression = true
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const argSize = 271828
	const respSize = 314159
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.Unimplemented {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code %s", err, codes.Unimplemented)
	}
	// Streaming RPC
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.Unimplemented {
		t.Fatalf("%v.Recv() = %v, want error code %s", stream, err, codes.Unimplemented)
	}
}

func TestCompressOK(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testCompressOK(t, e)
	}
}

func testCompressOK(t *testing.T, e env) {
	te := newTest(t, e)
	te.serverCompression = true
	te.clientCompression = true
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	// Unary call
	const argSize = 271828
	const respSize = 314159
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("something", "something"))
	if _, err := tc.UnaryCall(ctx, req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	// Streaming RPC
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: 31415,
		},
	}
	payload, err = newPayload(testpb.PayloadType_COMPRESSABLE, int32(31415))
	if err != nil {
		t.Fatal(err)
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	stream.CloseSend()
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v.Recv() = %v, want io.EOF", stream, err)
	}
}

func TestIdentityEncoding(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testIdentityEncoding(t, e)
	}
}

func testIdentityEncoding(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	// Unary call
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 5)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: 10,
		Payload:      payload,
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("something", "something"))
	if _, err := tc.UnaryCall(ctx, req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	// Streaming RPC
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := tc.FullDuplexCall(ctx, grpc.UseCompressor("identity"))
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	payload, err = newPayload(testpb.PayloadType_COMPRESSABLE, int32(31415))
	if err != nil {
		t.Fatal(err)
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: []*testpb.ResponseParameters{{Size: 10}},
		Payload:            payload,
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	stream.CloseSend()
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v.Recv() = %v, want io.EOF", stream, err)
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testUnaryClientInterceptor(t, e)
	}
}

func failOkayRPC(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err == nil {
		return status.Error(codes.NotFound, "")
	}
	return err
}

func testUnaryClientInterceptor(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.unaryClientInt = failOkayRPC
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	tc := testpb.NewTestServiceClient(te.clientConn())
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.NotFound {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, error code %s", tc, err, codes.NotFound)
	}
}

func TestStreamClientInterceptor(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testStreamClientInterceptor(t, e)
	}
}

func failOkayStream(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	s, err := streamer(ctx, desc, cc, method, opts...)
	if err == nil {
		return nil, status.Error(codes.NotFound, "")
	}
	return s, nil
}

func testStreamClientInterceptor(t *testing.T, e env) {
	te := newTest(t, e)
	te.streamClientInt = failOkayStream
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	tc := testpb.NewTestServiceClient(te.clientConn())
	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(1),
		},
	}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(1))
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	if _, err := tc.StreamingOutputCall(context.Background(), req); status.Code(err) != codes.NotFound {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want _, error code %s", tc, err, codes.NotFound)
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testUnaryServerInterceptor(t, e)
	}
}

func errInjector(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return nil, status.Error(codes.PermissionDenied, "")
}

func testUnaryServerInterceptor(t *testing.T, e env) {
	te := newTest(t, e)
	te.unaryServerInt = errInjector
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	tc := testpb.NewTestServiceClient(te.clientConn())
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("%v.EmptyCall(_, _) = _, %v, want _, error code %s", tc, err, codes.PermissionDenied)
	}
}

func TestStreamServerInterceptor(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		// TODO(bradfitz): Temporarily skip this env due to #619.
		if e.name == "handler-tls" {
			continue
		}
		testStreamServerInterceptor(t, e)
	}
}

func fullDuplexOnly(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if info.FullMethod == "/grpc.testing.TestService/FullDuplexCall" {
		return handler(srv, ss)
	}
	// Reject the other methods.
	return status.Error(codes.PermissionDenied, "")
}

func testStreamServerInterceptor(t *testing.T, e env) {
	te := newTest(t, e)
	te.streamServerInt = fullDuplexOnly
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	tc := testpb.NewTestServiceClient(te.clientConn())
	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(1),
		},
	}
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(1))
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	s1, err := tc.StreamingOutputCall(context.Background(), req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want _, <nil>", tc, err)
	}
	if _, err := s1.Recv(); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, error code %s", tc, err, codes.PermissionDenied)
	}
	s2, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := s2.Send(req); err != nil {
		t.Fatalf("%v.Send(_) = %v, want <nil>", s2, err)
	}
	if _, err := s2.Recv(); err != nil {
		t.Fatalf("%v.Recv() = _, %v, want _, <nil>", s2, err)
	}
}

// funcServer implements methods of TestServiceServer using funcs,
// similar to an http.HandlerFunc.
// Any unimplemented method will crash. Tests implement the method(s)
// they need.
type funcServer struct {
	testpb.TestServiceServer
	unaryCall          func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error)
	streamingInputCall func(stream testpb.TestService_StreamingInputCallServer) error
	fullDuplexCall     func(stream testpb.TestService_FullDuplexCallServer) error
}

func (s *funcServer) UnaryCall(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	return s.unaryCall(ctx, in)
}

func (s *funcServer) StreamingInputCall(stream testpb.TestService_StreamingInputCallServer) error {
	return s.streamingInputCall(stream)
}

func (s *funcServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	return s.fullDuplexCall(stream)
}

func TestClientRequestBodyErrorUnexpectedEOF(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testClientRequestBodyErrorUnexpectedEOF(t, e)
	}
}

func testClientRequestBodyErrorUnexpectedEOF(t *testing.T, e env) {
	te := newTest(t, e)
	ts := &funcServer{unaryCall: func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
		errUnexpectedCall := errors.New("unexpected call func server method")
		t.Error(errUnexpectedCall)
		return nil, errUnexpectedCall
	}}
	te.startServer(ts)
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/UnaryCall")
		// Say we have 5 bytes coming, but set END_STREAM flag:
		st.writeData(1, true, []byte{0, 0, 0, 0, 5})
		st.wantAnyFrame() // wait for server to crash (it used to crash)
	})
}

func TestClientRequestBodyErrorCloseAfterLength(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testClientRequestBodyErrorCloseAfterLength(t, e)
	}
}

func testClientRequestBodyErrorCloseAfterLength(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("Server.processUnaryRPC failed to write status")
	ts := &funcServer{unaryCall: func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
		errUnexpectedCall := errors.New("unexpected call func server method")
		t.Error(errUnexpectedCall)
		return nil, errUnexpectedCall
	}}
	te.startServer(ts)
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/UnaryCall")
		// say we're sending 5 bytes, but then close the connection instead.
		st.writeData(1, false, []byte{0, 0, 0, 0, 5})
		st.cc.Close()
	})
}

func TestClientRequestBodyErrorCancel(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testClientRequestBodyErrorCancel(t, e)
	}
}

func testClientRequestBodyErrorCancel(t *testing.T, e env) {
	te := newTest(t, e)
	gotCall := make(chan bool, 1)
	ts := &funcServer{unaryCall: func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
		gotCall <- true
		return new(testpb.SimpleResponse), nil
	}}
	te.startServer(ts)
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/UnaryCall")
		// Say we have 5 bytes coming, but cancel it instead.
		st.writeRSTStream(1, http2.ErrCodeCancel)
		st.writeData(1, false, []byte{0, 0, 0, 0, 5})

		// Verify we didn't a call yet.
		select {
		case <-gotCall:
			t.Fatal("unexpected call")
		default:
		}

		// And now send an uncanceled (but still invalid), just to get a response.
		st.writeHeadersGRPC(3, "/grpc.testing.TestService/UnaryCall")
		st.writeData(3, true, []byte{0, 0, 0, 0, 0})
		<-gotCall
		st.wantAnyFrame()
	})
}

func TestClientRequestBodyErrorCancelStreamingInput(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testClientRequestBodyErrorCancelStreamingInput(t, e)
	}
}

func testClientRequestBodyErrorCancelStreamingInput(t *testing.T, e env) {
	te := newTest(t, e)
	recvErr := make(chan error, 1)
	ts := &funcServer{streamingInputCall: func(stream testpb.TestService_StreamingInputCallServer) error {
		_, err := stream.Recv()
		recvErr <- err
		return nil
	}}
	te.startServer(ts)
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/StreamingInputCall")
		// Say we have 5 bytes coming, but cancel it instead.
		st.writeData(1, false, []byte{0, 0, 0, 0, 5})
		st.writeRSTStream(1, http2.ErrCodeCancel)

		var got error
		select {
		case got = <-recvErr:
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for error")
		}
		if grpc.Code(got) != codes.Canceled {
			t.Errorf("error = %#v; want error code %s", got, codes.Canceled)
		}
	})
}

func TestClientResourceExhaustedCancelFullDuplex(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.httpHandler {
			// httpHandler write won't be blocked on flow control window.
			continue
		}
		testClientResourceExhaustedCancelFullDuplex(t, e)
	}
}

func testClientResourceExhaustedCancelFullDuplex(t *testing.T, e env) {
	te := newTest(t, e)
	recvErr := make(chan error, 1)
	ts := &funcServer{fullDuplexCall: func(stream testpb.TestService_FullDuplexCallServer) error {
		defer close(recvErr)
		_, err := stream.Recv()
		if err != nil {
			return status.Errorf(codes.Internal, "stream.Recv() got error: %v, want <nil>", err)
		}
		// create a payload that's larger than the default flow control window.
		payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 10)
		if err != nil {
			return err
		}
		resp := &testpb.StreamingOutputCallResponse{
			Payload: payload,
		}
		ce := make(chan error)
		go func() {
			var err error
			for {
				if err = stream.Send(resp); err != nil {
					break
				}
			}
			ce <- err
		}()
		select {
		case err = <-ce:
		case <-time.After(10 * time.Second):
			err = errors.New("10s timeout reached")
		}
		recvErr <- err
		return err
	}}
	te.startServer(ts)
	defer te.tearDown()
	// set a low limit on receive message size to error with Resource Exhausted on
	// client side when server send a large message.
	te.maxClientReceiveMsgSize = newInt(10)
	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	req := &testpb.StreamingOutputCallRequest{}
	if err := stream.Send(req); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}
	err = <-recvErr
	if status.Code(err) != codes.Canceled {
		t.Fatalf("server got error %v, want error code: %s", err, codes.Canceled)
	}
}

type clientTimeoutCreds struct {
	timeoutReturned bool
}

func (c *clientTimeoutCreds) ClientHandshake(ctx context.Context, addr string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if !c.timeoutReturned {
		c.timeoutReturned = true
		return nil, nil, context.DeadlineExceeded
	}
	return rawConn, nil, nil
}
func (c *clientTimeoutCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}
func (c *clientTimeoutCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{}
}
func (c *clientTimeoutCreds) Clone() credentials.TransportCredentials {
	return nil
}
func (c *clientTimeoutCreds) OverrideServerName(s string) error {
	return nil
}

func TestNonFailFastRPCSucceedOnTimeoutCreds(t *testing.T) {
	te := newTest(t, env{name: "timeout-cred", network: "tcp", security: "clientTimeoutCreds", balancer: "v1"})
	te.userAgent = testAppUA
	te.startServer(&testServer{security: te.e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	// This unary call should succeed, because ClientHandshake will succeed for the second time.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); err != nil {
		te.t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want <nil>", err)
	}
}

type serverDispatchCred struct {
	rawConnCh chan net.Conn
}

func newServerDispatchCred() *serverDispatchCred {
	return &serverDispatchCred{
		rawConnCh: make(chan net.Conn, 1),
	}
}
func (c *serverDispatchCred) ClientHandshake(ctx context.Context, addr string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}
func (c *serverDispatchCred) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	select {
	case c.rawConnCh <- rawConn:
	default:
	}
	return nil, nil, credentials.ErrConnDispatched
}
func (c *serverDispatchCred) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{}
}
func (c *serverDispatchCred) Clone() credentials.TransportCredentials {
	return nil
}
func (c *serverDispatchCred) OverrideServerName(s string) error {
	return nil
}
func (c *serverDispatchCred) getRawConn() net.Conn {
	return <-c.rawConnCh
}

func TestServerCredsDispatch(t *testing.T) {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	cred := newServerDispatchCred()
	s := grpc.NewServer(grpc.Creds(cred))
	go s.Serve(lis)
	defer s.Stop()

	cc, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(cred))
	if err != nil {
		t.Fatalf("grpc.Dial(%q) = %v", lis.Addr().String(), err)
	}
	defer cc.Close()

	rawConn := cred.getRawConn()
	// Give grpc a chance to see the error and potentially close the connection.
	// And check that connection is not closed after that.
	time.Sleep(100 * time.Millisecond)
	// Check rawConn is not closed.
	if n, err := rawConn.Write([]byte{0}); n <= 0 || err != nil {
		t.Errorf("Read() = %v, %v; want n>0, <nil>", n, err)
	}
}

type authorityCheckCreds struct {
	got string
}

func (c *authorityCheckCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}
func (c *authorityCheckCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	c.got = authority
	return rawConn, nil, nil
}
func (c *authorityCheckCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{}
}
func (c *authorityCheckCreds) Clone() credentials.TransportCredentials {
	return c
}
func (c *authorityCheckCreds) OverrideServerName(s string) error {
	return nil
}

// This test makes sure that the authority client handshake gets is the endpoint
// in dial target, not the resolved ip address.
func TestCredsHandshakeAuthority(t *testing.T) {
	const testAuthority = "test.auth.ori.ty"

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	cred := &authorityCheckCreds{}
	s := grpc.NewServer()
	go s.Serve(lis)
	defer s.Stop()

	r, rcleanup := manual.GenerateAndRegisterManualResolver()
	defer rcleanup()

	cc, err := grpc.Dial(r.Scheme()+":///"+testAuthority, grpc.WithTransportCredentials(cred))
	if err != nil {
		t.Fatalf("grpc.Dial(%q) = %v", lis.Addr().String(), err)
	}
	defer cc.Close()
	r.NewAddress([]resolver.Address{{Addr: lis.Addr().String()}})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	for {
		s := cc.GetState()
		if s == connectivity.Ready {
			break
		}
		if !cc.WaitForStateChange(ctx, s) {
			// ctx got timeout or canceled.
			t.Fatalf("ClientConn is not ready after 100 ms")
		}
	}

	if cred.got != testAuthority {
		t.Fatalf("client creds got authority: %q, want: %q", cred.got, testAuthority)
	}
}

type clientFailCreds struct {
	got string
}

func (c *clientFailCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}
func (c *clientFailCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, fmt.Errorf("client handshake fails with fatal error")
}
func (c *clientFailCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{}
}
func (c *clientFailCreds) Clone() credentials.TransportCredentials {
	return c
}
func (c *clientFailCreds) OverrideServerName(s string) error {
	return nil
}

// This test makes sure that failfast RPCs fail if client handshake fails with
// fatal errors.
func TestFailfastRPCFailOnFatalHandshakeError(t *testing.T) {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	cc, err := grpc.Dial("passthrough:///"+lis.Addr().String(), grpc.WithTransportCredentials(&clientFailCreds{}))
	if err != nil {
		t.Fatalf("grpc.Dial(_) = %v", err)
	}
	defer cc.Close()

	tc := testpb.NewTestServiceClient(cc)
	// This unary call should fail, but not timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(true)); status.Code(err) != codes.Unavailable {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want <Unavailable>", err)
	}
}

func TestFlowControlLogicalRace(t *testing.T) {
	// Test for a regression of https://github.com/grpc/grpc-go/issues/632,
	// and other flow control bugs.

	defer leakcheck.Check(t)

	const (
		itemCount   = 100
		itemSize    = 1 << 10
		recvCount   = 2
		maxFailures = 3

		requestTimeout = time.Second * 5
	)

	requestCount := 10000
	if raceMode {
		requestCount = 1000
	}

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	testpb.RegisterTestServiceServer(s, &flowControlLogicalRaceServer{
		itemCount: itemCount,
		itemSize:  itemSize,
	})
	defer s.Stop()

	go s.Serve(lis)

	ctx := context.Background()

	cc, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("grpc.Dial(%q) = %v", lis.Addr().String(), err)
	}
	defer cc.Close()
	cl := testpb.NewTestServiceClient(cc)

	failures := 0
	for i := 0; i < requestCount; i++ {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		output, err := cl.StreamingOutputCall(ctx, &testpb.StreamingOutputCallRequest{})
		if err != nil {
			t.Fatalf("StreamingOutputCall; err = %q", err)
		}

		j := 0
	loop:
		for ; j < recvCount; j++ {
			_, err := output.Recv()
			if err != nil {
				if err == io.EOF {
					break loop
				}
				switch status.Code(err) {
				case codes.DeadlineExceeded:
					break loop
				default:
					t.Fatalf("Recv; err = %q", err)
				}
			}
		}
		cancel()
		<-ctx.Done()

		if j < recvCount {
			t.Errorf("got %d responses to request %d", j, i)
			failures++
			if failures >= maxFailures {
				// Continue past the first failure to see if the connection is
				// entirely broken, or if only a single RPC was affected
				break
			}
		}
	}
}

type flowControlLogicalRaceServer struct {
	testpb.TestServiceServer

	itemSize  int
	itemCount int
}

func (s *flowControlLogicalRaceServer) StreamingOutputCall(req *testpb.StreamingOutputCallRequest, srv testpb.TestService_StreamingOutputCallServer) error {
	for i := 0; i < s.itemCount; i++ {
		err := srv.Send(&testpb.StreamingOutputCallResponse{
			Payload: &testpb.Payload{
				// Sending a large stream of data which the client reject
				// helps to trigger some types of flow control bugs.
				//
				// Reallocating memory here is inefficient, but the stress it
				// puts on the GC leads to more frequent flow control
				// failures. The GC likely causes more variety in the
				// goroutine scheduling orders.
				Body: bytes.Repeat([]byte("a"), s.itemSize),
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type lockingWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockingWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

func (lw *lockingWriter) setWriter(w io.Writer) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.w = w
}

var testLogOutput = &lockingWriter{w: os.Stderr}

// awaitNewConnLogOutput waits for any of grpc.NewConn's goroutines to
// terminate, if they're still running. It spams logs with this
// message.  We wait for it so our log filter is still
// active. Otherwise the "defer restore()" at the top of various test
// functions restores our log filter and then the goroutine spams.
func awaitNewConnLogOutput() {
	awaitLogOutput(50*time.Millisecond, "grpc: the client connection is closing; please retry")
}

func awaitLogOutput(maxWait time.Duration, phrase string) {
	pb := []byte(phrase)

	timer := time.NewTimer(maxWait)
	defer timer.Stop()
	wakeup := make(chan bool, 1)
	for {
		if logOutputHasContents(pb, wakeup) {
			return
		}
		select {
		case <-timer.C:
			// Too slow. Oh well.
			return
		case <-wakeup:
		}
	}
}

func logOutputHasContents(v []byte, wakeup chan<- bool) bool {
	testLogOutput.mu.Lock()
	defer testLogOutput.mu.Unlock()
	fw, ok := testLogOutput.w.(*filterWriter)
	if !ok {
		return false
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if bytes.Contains(fw.buf.Bytes(), v) {
		return true
	}
	fw.wakeup = wakeup
	return false
}

var verboseLogs = flag.Bool("verbose_logs", false, "show all grpclog output, without filtering")

func noop() {}

// declareLogNoise declares that t is expected to emit the following noisy phrases,
// even on success. Those phrases will be filtered from grpclog output
// and only be shown if *verbose_logs or t ends up failing.
// The returned restore function should be called with defer to be run
// before the test ends.
func declareLogNoise(t *testing.T, phrases ...string) (restore func()) {
	if *verboseLogs {
		return noop
	}
	fw := &filterWriter{dst: os.Stderr, filter: phrases}
	testLogOutput.setWriter(fw)
	return func() {
		if t.Failed() {
			fw.mu.Lock()
			defer fw.mu.Unlock()
			if fw.buf.Len() > 0 {
				t.Logf("Complete log output:\n%s", fw.buf.Bytes())
			}
		}
		testLogOutput.setWriter(os.Stderr)
	}
}

type filterWriter struct {
	dst    io.Writer
	filter []string

	mu     sync.Mutex
	buf    bytes.Buffer
	wakeup chan<- bool // if non-nil, gets true on write
}

func (fw *filterWriter) Write(p []byte) (n int, err error) {
	fw.mu.Lock()
	fw.buf.Write(p)
	if fw.wakeup != nil {
		select {
		case fw.wakeup <- true:
		default:
		}
	}
	fw.mu.Unlock()

	ps := string(p)
	for _, f := range fw.filter {
		if strings.Contains(ps, f) {
			return len(p), nil
		}
	}
	return fw.dst.Write(p)
}

// stubServer is a server that is easy to customize within individual test
// cases.
type stubServer struct {
	// Guarantees we satisfy this interface; panics if unimplemented methods are called.
	testpb.TestServiceServer

	// Customizable implementations of server handlers.
	emptyCall      func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error)
	fullDuplexCall func(stream testpb.TestService_FullDuplexCallServer) error

	// A client connected to this service the test may use.  Created in Start().
	client testpb.TestServiceClient

	cleanups []func() // Lambdas executed in Stop(); populated by Start().
}

func (ss *stubServer) EmptyCall(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
	return ss.emptyCall(ctx, in)
}

func (ss *stubServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	return ss.fullDuplexCall(stream)
}

// Start starts the server and creates a client connected to it.
func (ss *stubServer) Start(sopts []grpc.ServerOption) error {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf(`net.Listen("tcp", "localhost:0") = %v`, err)
	}
	ss.cleanups = append(ss.cleanups, func() { lis.Close() })

	s := grpc.NewServer(sopts...)
	testpb.RegisterTestServiceServer(s, ss)
	go s.Serve(lis)
	ss.cleanups = append(ss.cleanups, s.Stop)

	cc, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("grpc.Dial(%q) = %v", lis.Addr().String(), err)
	}
	ss.cleanups = append(ss.cleanups, func() { cc.Close() })

	ss.client = testpb.NewTestServiceClient(cc)
	return nil
}

func (ss *stubServer) Stop() {
	for i := len(ss.cleanups) - 1; i >= 0; i-- {
		ss.cleanups[i]()
	}
}

func TestGRPCMethod(t *testing.T) {
	defer leakcheck.Check(t)
	var method string
	var ok bool

	ss := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			method, ok = grpc.Method(ctx)
			return &testpb.Empty{}, nil
		},
	}
	if err := ss.Start(nil); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer ss.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := ss.client.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatalf("ss.client.EmptyCall(_, _) = _, %v; want _, nil", err)
	}

	if want := "/grpc.testing.TestService/EmptyCall"; !ok || method != want {
		t.Fatalf("grpc.Method(_) = %q, %v; want %q, true", method, ok, want)
	}
}

func TestUnaryProxyDoesNotForwardMetadata(t *testing.T) {
	const mdkey = "somedata"

	// endpoint ensures mdkey is NOT in metadata and returns an error if it is.
	endpoint := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			if md, ok := metadata.FromIncomingContext(ctx); !ok || md[mdkey] != nil {
				return nil, status.Errorf(codes.Internal, "endpoint: md=%v; want !contains(%q)", md, mdkey)
			}
			return &testpb.Empty{}, nil
		},
	}
	if err := endpoint.Start(nil); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer endpoint.Stop()

	// proxy ensures mdkey IS in metadata, then forwards the RPC to endpoint
	// without explicitly copying the metadata.
	proxy := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			if md, ok := metadata.FromIncomingContext(ctx); !ok || md[mdkey] == nil {
				return nil, status.Errorf(codes.Internal, "proxy: md=%v; want contains(%q)", md, mdkey)
			}
			return endpoint.client.EmptyCall(ctx, in)
		},
	}
	if err := proxy.Start(nil); err != nil {
		t.Fatalf("Error starting proxy server: %v", err)
	}
	defer proxy.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	md := metadata.Pairs(mdkey, "val")
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Sanity check that endpoint properly errors when it sees mdkey.
	_, err := endpoint.client.EmptyCall(ctx, &testpb.Empty{})
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
		t.Fatalf("endpoint.client.EmptyCall(_, _) = _, %v; want _, <status with Code()=Internal>", err)
	}

	if _, err := proxy.client.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatal(err.Error())
	}
}

func TestStreamingProxyDoesNotForwardMetadata(t *testing.T) {
	const mdkey = "somedata"

	// doFDC performs a FullDuplexCall with client and returns the error from the
	// first stream.Recv call, or nil if that error is io.EOF.  Calls t.Fatal if
	// the stream cannot be established.
	doFDC := func(ctx context.Context, client testpb.TestServiceClient) error {
		stream, err := client.FullDuplexCall(ctx)
		if err != nil {
			t.Fatalf("Unwanted error: %v", err)
		}
		if _, err := stream.Recv(); err != io.EOF {
			return err
		}
		return nil
	}

	// endpoint ensures mdkey is NOT in metadata and returns an error if it is.
	endpoint := &stubServer{
		fullDuplexCall: func(stream testpb.TestService_FullDuplexCallServer) error {
			ctx := stream.Context()
			if md, ok := metadata.FromIncomingContext(ctx); !ok || md[mdkey] != nil {
				return status.Errorf(codes.Internal, "endpoint: md=%v; want !contains(%q)", md, mdkey)
			}
			return nil
		},
	}
	if err := endpoint.Start(nil); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer endpoint.Stop()

	// proxy ensures mdkey IS in metadata, then forwards the RPC to endpoint
	// without explicitly copying the metadata.
	proxy := &stubServer{
		fullDuplexCall: func(stream testpb.TestService_FullDuplexCallServer) error {
			ctx := stream.Context()
			if md, ok := metadata.FromIncomingContext(ctx); !ok || md[mdkey] == nil {
				return status.Errorf(codes.Internal, "endpoint: md=%v; want !contains(%q)", md, mdkey)
			}
			return doFDC(ctx, endpoint.client)
		},
	}
	if err := proxy.Start(nil); err != nil {
		t.Fatalf("Error starting proxy server: %v", err)
	}
	defer proxy.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	md := metadata.Pairs(mdkey, "val")
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Sanity check that endpoint properly errors when it sees mdkey in ctx.
	err := doFDC(ctx, endpoint.client)
	if s, ok := status.FromError(err); !ok || s.Code() != codes.Internal {
		t.Fatalf("stream.Recv() = _, %v; want _, <status with Code()=Internal>", err)
	}

	if err := doFDC(ctx, proxy.client); err != nil {
		t.Fatalf("doFDC(_, proxy.client) = %v; want nil", err)
	}
}

func TestStatsTagsAndTrace(t *testing.T) {
	// Data added to context by client (typically in a stats handler).
	tags := []byte{1, 5, 2, 4, 3}
	trace := []byte{5, 2, 1, 3, 4}

	// endpoint ensures Tags() and Trace() in context match those that were added
	// by the client and returns an error if not.
	endpoint := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			md, _ := metadata.FromIncomingContext(ctx)
			if tg := stats.Tags(ctx); !reflect.DeepEqual(tg, tags) {
				return nil, status.Errorf(codes.Internal, "stats.Tags(%v)=%v; want %v", ctx, tg, tags)
			}
			if !reflect.DeepEqual(md["grpc-tags-bin"], []string{string(tags)}) {
				return nil, status.Errorf(codes.Internal, "md['grpc-tags-bin']=%v; want %v", md["grpc-tags-bin"], tags)
			}
			if tr := stats.Trace(ctx); !reflect.DeepEqual(tr, trace) {
				return nil, status.Errorf(codes.Internal, "stats.Trace(%v)=%v; want %v", ctx, tr, trace)
			}
			if !reflect.DeepEqual(md["grpc-trace-bin"], []string{string(trace)}) {
				return nil, status.Errorf(codes.Internal, "md['grpc-trace-bin']=%v; want %v", md["grpc-trace-bin"], trace)
			}
			return &testpb.Empty{}, nil
		},
	}
	if err := endpoint.Start(nil); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer endpoint.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	testCases := []struct {
		ctx  context.Context
		want codes.Code
	}{
		{ctx: ctx, want: codes.Internal},
		{ctx: stats.SetTags(ctx, tags), want: codes.Internal},
		{ctx: stats.SetTrace(ctx, trace), want: codes.Internal},
		{ctx: stats.SetTags(stats.SetTrace(ctx, tags), tags), want: codes.Internal},
		{ctx: stats.SetTags(stats.SetTrace(ctx, trace), tags), want: codes.OK},
	}

	for _, tc := range testCases {
		_, err := endpoint.client.EmptyCall(tc.ctx, &testpb.Empty{})
		if tc.want == codes.OK && err != nil {
			t.Fatalf("endpoint.client.EmptyCall(%v, _) = _, %v; want _, nil", tc.ctx, err)
		}
		if s, ok := status.FromError(err); !ok || s.Code() != tc.want {
			t.Fatalf("endpoint.client.EmptyCall(%v, _) = _, %v; want _, <status with Code()=%v>", tc.ctx, err, tc.want)
		}
	}
}

func TestTapTimeout(t *testing.T) {
	sopts := []grpc.ServerOption{
		grpc.InTapHandle(func(ctx context.Context, _ *tap.Info) (context.Context, error) {
			c, cancel := context.WithCancel(ctx)
			// Call cancel instead of setting a deadline so we can detect which error
			// occurred -- this cancellation (desired) or the client's deadline
			// expired (indicating this cancellation did not affect the RPC).
			time.AfterFunc(10*time.Millisecond, cancel)
			return c, nil
		}),
	}

	ss := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			<-ctx.Done()
			return nil, status.Errorf(codes.Canceled, ctx.Err().Error())
		},
	}
	if err := ss.Start(sopts); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer ss.Stop()

	// This was known to be flaky; test several times.
	for i := 0; i < 10; i++ {
		// Set our own deadline in case the server hangs.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		res, err := ss.client.EmptyCall(ctx, &testpb.Empty{})
		cancel()
		if s, ok := status.FromError(err); !ok || s.Code() != codes.Canceled {
			t.Fatalf("ss.client.EmptyCall(context.Background(), _) = %v, %v; want nil, <status with Code()=Canceled>", res, err)
		}
	}

}

func TestClientWriteFailsAfterServerClosesStream(t *testing.T) {
	ss := &stubServer{
		fullDuplexCall: func(stream testpb.TestService_FullDuplexCallServer) error {
			return status.Errorf(codes.Internal, "")
		},
	}
	sopts := []grpc.ServerOption{}
	if err := ss.Start(sopts); err != nil {
		t.Fatalf("Error starting endpoing server: %v", err)
	}
	defer ss.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := ss.client.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("Error while creating stream: %v", err)
	}
	for {
		if err := stream.Send(&testpb.StreamingOutputCallRequest{}); err == nil {
			time.Sleep(5 * time.Millisecond)
		} else if err == io.EOF {
			break // Success.
		} else {
			t.Fatalf("stream.Send(_) = %v, want io.EOF", err)
		}
	}

}

type windowSizeConfig struct {
	serverStream int32
	serverConn   int32
	clientStream int32
	clientConn   int32
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func TestConfigurableWindowSizeWithLargeWindow(t *testing.T) {
	defer leakcheck.Check(t)
	wc := windowSizeConfig{
		serverStream: 8 * 1024 * 1024,
		serverConn:   12 * 1024 * 1024,
		clientStream: 6 * 1024 * 1024,
		clientConn:   8 * 1024 * 1024,
	}
	for _, e := range listTestEnv() {
		testConfigurableWindowSize(t, e, wc)
	}
}

func TestConfigurableWindowSizeWithSmallWindow(t *testing.T) {
	defer leakcheck.Check(t)
	wc := windowSizeConfig{
		serverStream: 1,
		serverConn:   1,
		clientStream: 1,
		clientConn:   1,
	}
	for _, e := range listTestEnv() {
		testConfigurableWindowSize(t, e, wc)
	}
}

func testConfigurableWindowSize(t *testing.T, e env, wc windowSizeConfig) {
	te := newTest(t, e)
	te.serverInitialWindowSize = wc.serverStream
	te.serverInitialConnWindowSize = wc.serverConn
	te.clientInitialWindowSize = wc.clientStream
	te.clientInitialConnWindowSize = wc.clientConn

	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	numOfIter := 11
	// Set message size to exhaust largest of window sizes.
	messageSize := max(max(wc.serverStream, wc.serverConn), max(wc.clientStream, wc.clientConn)) / int32(numOfIter-1)
	messageSize = max(messageSize, 64*1024)
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, messageSize)
	if err != nil {
		t.Fatal(err)
	}
	respParams := []*testpb.ResponseParameters{
		{
			Size: messageSize,
		},
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParams,
		Payload:            payload,
	}
	for i := 0; i < numOfIter; i++ {
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
		}
		if _, err := stream.Recv(); err != nil {
			t.Fatalf("%v.Recv() = _, %v, want _, <nil>", stream, err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() = %v, want <nil>", stream, err)
	}
}

var (
	// test authdata
	authdata = map[string]string{
		"test-key":      "test-value",
		"test-key2-bin": string([]byte{1, 2, 3}),
	}
)

type testPerRPCCredentials struct{}

func (cr testPerRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return authdata, nil
}

func (cr testPerRPCCredentials) RequireTransportSecurity() bool {
	return false
}

func authHandle(ctx context.Context, info *tap.Info) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, fmt.Errorf("didn't find metadata in context")
	}
	for k, vwant := range authdata {
		vgot, ok := md[k]
		if !ok {
			return ctx, fmt.Errorf("didn't find authdata key %v in context", k)
		}
		if vgot[0] != vwant {
			return ctx, fmt.Errorf("for key %v, got value %v, want %v", k, vgot, vwant)
		}
	}
	return ctx, nil
}

func TestPerRPCCredentialsViaDialOptions(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPerRPCCredentialsViaDialOptions(t, e)
	}
}

func testPerRPCCredentialsViaDialOptions(t *testing.T, e env) {
	te := newTest(t, e)
	te.tapHandle = authHandle
	te.perRPCCreds = testPerRPCCredentials{}
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("Test failed. Reason: %v", err)
	}
}

func TestPerRPCCredentialsViaCallOptions(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPerRPCCredentialsViaCallOptions(t, e)
	}
}

func testPerRPCCredentialsViaCallOptions(t *testing.T, e env) {
	te := newTest(t, e)
	te.tapHandle = authHandle
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.PerRPCCredentials(testPerRPCCredentials{})); err != nil {
		t.Fatalf("Test failed. Reason: %v", err)
	}
}

func TestPerRPCCredentialsViaDialOptionsAndCallOptions(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testPerRPCCredentialsViaDialOptionsAndCallOptions(t, e)
	}
}

func testPerRPCCredentialsViaDialOptionsAndCallOptions(t *testing.T, e env) {
	te := newTest(t, e)
	te.perRPCCreds = testPerRPCCredentials{}
	// When credentials are provided via both dial options and call options,
	// we apply both sets.
	te.tapHandle = func(ctx context.Context, _ *tap.Info) (context.Context, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return ctx, fmt.Errorf("couldn't find metadata in context")
		}
		for k, vwant := range authdata {
			vgot, ok := md[k]
			if !ok {
				return ctx, fmt.Errorf("couldn't find metadata for key %v", k)
			}
			if len(vgot) != 2 {
				return ctx, fmt.Errorf("len of value for key %v was %v, want 2", k, len(vgot))
			}
			if vgot[0] != vwant || vgot[1] != vwant {
				return ctx, fmt.Errorf("value for %v was %v, want [%v, %v]", k, vgot, vwant, vwant)
			}
		}
		return ctx, nil
	}
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.PerRPCCredentials(testPerRPCCredentials{})); err != nil {
		t.Fatalf("Test failed. Reason: %v", err)
	}
}

func TestWaitForReadyConnection(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testWaitForReadyConnection(t, e)
	}

}

func testWaitForReadyConnection(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	cc := te.clientConn() // Non-blocking dial.
	tc := testpb.NewTestServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	state := cc.GetState()
	// Wait for connection to be Ready.
	for ; state != connectivity.Ready && cc.WaitForStateChange(ctx, state); state = cc.GetState() {
	}
	if state != connectivity.Ready {
		t.Fatalf("Want connection state to be Ready, got %v", state)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Make a fail-fast RPC.
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatalf("TestService/EmptyCall(_,_) = _, %v, want _, nil", err)
	}
}

type errCodec struct {
	noError bool
}

func (c *errCodec) Marshal(v interface{}) ([]byte, error) {
	if c.noError {
		return []byte{}, nil
	}
	return nil, fmt.Errorf("3987^12 + 4365^12 = 4472^12")
}

func (c *errCodec) Unmarshal(data []byte, v interface{}) error {
	return nil
}

func (c *errCodec) String() string {
	return "Fermat's near-miss."
}

func TestEncodeDoesntPanic(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testEncodeDoesntPanic(t, e)
	}
}

func testEncodeDoesntPanic(t *testing.T, e env) {
	te := newTest(t, e)
	erc := &errCodec{}
	te.customCodec = erc
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	te.customCodec = nil
	tc := testpb.NewTestServiceClient(te.clientConn())
	// Failure case, should not panic.
	tc.EmptyCall(context.Background(), &testpb.Empty{})
	erc.noError = true
	// Passing case.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); err != nil {
		t.Fatalf("EmptyCall(_, _) = _, %v, want _, <nil>", err)
	}
}

func TestSvrWriteStatusEarlyWrite(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testSvrWriteStatusEarlyWrite(t, e)
	}
}

func testSvrWriteStatusEarlyWrite(t *testing.T, e env) {
	te := newTest(t, e)
	const smallSize = 1024
	const largeSize = 2048
	const extraLargeSize = 4096
	te.maxServerReceiveMsgSize = newInt(largeSize)
	te.maxServerSendMsgSize = newInt(largeSize)
	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}
	extraLargePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, extraLargeSize)
	if err != nil {
		t.Fatal(err)
	}
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())
	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(smallSize),
		},
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            extraLargePayload,
	}
	// Test recv case: server receives a message larger than maxServerReceiveMsgSize.
	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err = stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send() = _, %v, want <nil>", stream, err)
	}
	if _, err = stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}
	// Test send case: server sends a message larger than maxServerSendMsgSize.
	sreq.Payload = smallPayload
	respParam[0].Size = int32(extraLargeSize)

	stream, err = tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err = stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err = stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}
}

// The following functions with function name ending with TD indicates that they
// should be deleted after old service config API is deprecated and deleted.
func testServiceConfigSetupTD(t *testing.T, e env) (*test, chan grpc.ServiceConfig) {
	te := newTest(t, e)
	// We write before read.
	ch := make(chan grpc.ServiceConfig, 1)
	te.sc = ch
	te.userAgent = testAppUA
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: addrConn.transportMonitor exits due to: grpc: the connection is closing",
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"Failed to dial : context canceled; please retry.",
	)
	return te, ch
}

func TestServiceConfigGetMethodConfigTD(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testGetMethodConfigTD(t, e)
	}
}

func testGetMethodConfigTD(t *testing.T, e env) {
	te, ch := testServiceConfigSetupTD(t, e)
	defer te.tearDown()

	mc1 := grpc.MethodConfig{
		WaitForReady: newBool(true),
		Timeout:      newDuration(time.Millisecond),
	}
	mc2 := grpc.MethodConfig{WaitForReady: newBool(false)}
	m := make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/EmptyCall"] = mc1
	m["/grpc.testing.TestService/"] = mc2
	sc := grpc.ServiceConfig{
		Methods: m,
	}
	ch <- sc

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	// The following RPCs are expected to become non-fail-fast ones with 1ms deadline.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}

	m = make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/UnaryCall"] = mc1
	m["/grpc.testing.TestService/"] = mc2
	sc = grpc.ServiceConfig{
		Methods: m,
	}
	ch <- sc
	// Wait for the new service config to propagate.
	for {
		if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) == codes.DeadlineExceeded {
			continue
		}
		break
	}
	// The following RPCs are expected to become fail-fast.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.Unavailable)
	}
}

func TestServiceConfigWaitForReadyTD(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testServiceConfigWaitForReadyTD(t, e)
	}
}

func testServiceConfigWaitForReadyTD(t *testing.T, e env) {
	te, ch := testServiceConfigSetupTD(t, e)
	defer te.tearDown()

	// Case1: Client API set failfast to be false, and service config set wait_for_ready to be false, Client API should win, and the rpc will wait until deadline exceeds.
	mc := grpc.MethodConfig{
		WaitForReady: newBool(false),
		Timeout:      newDuration(time.Millisecond),
	}
	m := make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/EmptyCall"] = mc
	m["/grpc.testing.TestService/FullDuplexCall"] = mc
	sc := grpc.ServiceConfig{
		Methods: m,
	}
	ch <- sc

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	// The following RPCs are expected to become non-fail-fast ones with 1ms deadline.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	if _, err := tc.FullDuplexCall(context.Background(), grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}

	// Generate a service config update.
	// Case2: Client API does not set failfast, and service config set wait_for_ready to be true, and the rpc will wait until deadline exceeds.
	mc.WaitForReady = newBool(true)
	m = make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/EmptyCall"] = mc
	m["/grpc.testing.TestService/FullDuplexCall"] = mc
	sc = grpc.ServiceConfig{
		Methods: m,
	}
	ch <- sc

	// Wait for the new service config to take effect.
	mc = cc.GetMethodConfig("/grpc.testing.TestService/EmptyCall")
	for {
		if !*mc.WaitForReady {
			time.Sleep(100 * time.Millisecond)
			mc = cc.GetMethodConfig("/grpc.testing.TestService/EmptyCall")
			continue
		}
		break
	}
	// The following RPCs are expected to become non-fail-fast ones with 1ms deadline.
	if _, err := tc.EmptyCall(context.Background(), &testpb.Empty{}); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	if _, err := tc.FullDuplexCall(context.Background()); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}
}

func TestServiceConfigTimeoutTD(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testServiceConfigTimeoutTD(t, e)
	}
}

func testServiceConfigTimeoutTD(t *testing.T, e env) {
	te, ch := testServiceConfigSetupTD(t, e)
	defer te.tearDown()

	// Case1: Client API sets timeout to be 1ns and ServiceConfig sets timeout to be 1hr. Timeout should be 1ns (min of 1ns and 1hr) and the rpc will wait until deadline exceeds.
	mc := grpc.MethodConfig{
		Timeout: newDuration(time.Hour),
	}
	m := make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/EmptyCall"] = mc
	m["/grpc.testing.TestService/FullDuplexCall"] = mc
	sc := grpc.ServiceConfig{
		Methods: m,
	}
	ch <- sc

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	// The following RPCs are expected to become non-fail-fast ones with 1ns deadline.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), time.Nanosecond)
	if _, err := tc.FullDuplexCall(ctx, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}
	cancel()

	// Generate a service config update.
	// Case2: Client API sets timeout to be 1hr and ServiceConfig sets timeout to be 1ns. Timeout should be 1ns (min of 1ns and 1hr) and the rpc will wait until deadline exceeds.
	mc.Timeout = newDuration(time.Nanosecond)
	m = make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/EmptyCall"] = mc
	m["/grpc.testing.TestService/FullDuplexCall"] = mc
	sc = grpc.ServiceConfig{
		Methods: m,
	}
	ch <- sc

	// Wait for the new service config to take effect.
	mc = cc.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall")
	for {
		if *mc.Timeout != time.Nanosecond {
			time.Sleep(100 * time.Millisecond)
			mc = cc.GetMethodConfig("/grpc.testing.TestService/FullDuplexCall")
			continue
		}
		break
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %s", err, codes.DeadlineExceeded)
	}
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), time.Hour)
	if _, err := tc.FullDuplexCall(ctx, grpc.FailFast(false)); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/FullDuplexCall(_) = _, %v, want %s", err, codes.DeadlineExceeded)
	}
	cancel()
}

func TestServiceConfigMaxMsgSizeTD(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testServiceConfigMaxMsgSizeTD(t, e)
	}
}

func testServiceConfigMaxMsgSizeTD(t *testing.T, e env) {
	// Setting up values and objects shared across all test cases.
	const smallSize = 1
	const largeSize = 1024
	const extraLargeSize = 2048

	smallPayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, smallSize)
	if err != nil {
		t.Fatal(err)
	}
	largePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, largeSize)
	if err != nil {
		t.Fatal(err)
	}
	extraLargePayload, err := newPayload(testpb.PayloadType_COMPRESSABLE, extraLargeSize)
	if err != nil {
		t.Fatal(err)
	}

	mc := grpc.MethodConfig{
		MaxReqSize:  newInt(extraLargeSize),
		MaxRespSize: newInt(extraLargeSize),
	}

	m := make(map[string]grpc.MethodConfig)
	m["/grpc.testing.TestService/UnaryCall"] = mc
	m["/grpc.testing.TestService/FullDuplexCall"] = mc
	sc := grpc.ServiceConfig{
		Methods: m,
	}
	// Case1: sc set maxReqSize to 2048 (send), maxRespSize to 2048 (recv).
	te1, ch1 := testServiceConfigSetupTD(t, e)
	te1.startServer(&testServer{security: e.security})
	defer te1.tearDown()

	ch1 <- sc
	tc := testpb.NewTestServiceClient(te1.clientConn())

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: int32(extraLargeSize),
		Payload:      smallPayload,
	}
	// Test for unary RPC recv.
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = extraLargePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	respParam := []*testpb.ResponseParameters{
		{
			Size: int32(extraLargeSize),
		},
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            smallPayload,
	}
	stream, err := tc.FullDuplexCall(te1.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = extraLargePayload
	stream, err = tc.FullDuplexCall(te1.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}

	// Case2: Client API set maxReqSize to 1024 (send), maxRespSize to 1024 (recv). Sc sets maxReqSize to 2048 (send), maxRespSize to 2048 (recv).
	te2, ch2 := testServiceConfigSetupTD(t, e)
	te2.maxClientReceiveMsgSize = newInt(1024)
	te2.maxClientSendMsgSize = newInt(1024)
	te2.startServer(&testServer{security: e.security})
	defer te2.tearDown()
	ch2 <- sc
	tc = testpb.NewTestServiceClient(te2.clientConn())

	// Test for unary RPC recv.
	req.Payload = smallPayload
	req.ResponseSize = int32(largeSize)

	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = largePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	stream, err = tc.FullDuplexCall(te2.ctx)
	respParam[0].Size = int32(largeSize)
	sreq.Payload = smallPayload
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = largePayload
	stream, err = tc.FullDuplexCall(te2.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}

	// Case3: Client API set maxReqSize to 4096 (send), maxRespSize to 4096 (recv). Sc sets maxReqSize to 2048 (send), maxRespSize to 2048 (recv).
	te3, ch3 := testServiceConfigSetupTD(t, e)
	te3.maxClientReceiveMsgSize = newInt(4096)
	te3.maxClientSendMsgSize = newInt(4096)
	te3.startServer(&testServer{security: e.security})
	defer te3.tearDown()
	ch3 <- sc
	tc = testpb.NewTestServiceClient(te3.clientConn())

	// Test for unary RPC recv.
	req.Payload = smallPayload
	req.ResponseSize = int32(largeSize)

	if _, err := tc.UnaryCall(context.Background(), req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want <nil>", err)
	}

	req.ResponseSize = int32(extraLargeSize)
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for unary RPC send.
	req.Payload = largePayload
	req.ResponseSize = int32(smallSize)
	if _, err := tc.UnaryCall(context.Background(), req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want <nil>", err)
	}

	req.Payload = extraLargePayload
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code: %s", err, codes.ResourceExhausted)
	}

	// Test for streaming RPC recv.
	stream, err = tc.FullDuplexCall(te3.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam[0].Size = int32(largeSize)
	sreq.Payload = smallPayload

	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = _, %v, want <nil>", stream, err)
	}

	respParam[0].Size = int32(extraLargeSize)

	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Recv() = _, %v, want _, error code: %s", stream, err, codes.ResourceExhausted)
	}

	// Test for streaming RPC send.
	respParam[0].Size = int32(smallSize)
	sreq.Payload = largePayload
	stream, err = tc.FullDuplexCall(te3.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	sreq.Payload = extraLargePayload
	if err := stream.Send(sreq); err == nil || status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("%v.Send(%v) = %v, want _, error code: %s", stream, sreq, err, codes.ResourceExhausted)
	}
}

func TestMethodFromServerStream(t *testing.T) {
	defer leakcheck.Check(t)
	const testMethod = "/package.service/method"
	e := tcpClearRREnv
	te := newTest(t, e)
	var method string
	var ok bool
	te.unknownHandler = func(srv interface{}, stream grpc.ServerStream) error {
		method, ok = grpc.MethodFromServerStream(stream)
		return nil
	}

	te.startServer(nil)
	defer te.tearDown()
	_ = te.clientConn().Invoke(context.Background(), testMethod, nil, nil)
	if !ok || method != testMethod {
		t.Fatalf("Invoke with method %q, got %q, %v, want %q, true", testMethod, method, ok, testMethod)
	}
}

func TestInterceptorCanAccessCallOptions(t *testing.T) {
	defer leakcheck.Check(t)
	e := tcpClearRREnv
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()

	type observedOptions struct {
		headers     []*metadata.MD
		trailers    []*metadata.MD
		peer        []*peer.Peer
		creds       []credentials.PerRPCCredentials
		failFast    []bool
		maxRecvSize []int
		maxSendSize []int
		compressor  []string
		subtype     []string
	}
	var observedOpts observedOptions
	populateOpts := func(opts []grpc.CallOption) {
		for _, o := range opts {
			switch o := o.(type) {
			case grpc.HeaderCallOption:
				observedOpts.headers = append(observedOpts.headers, o.HeaderAddr)
			case grpc.TrailerCallOption:
				observedOpts.trailers = append(observedOpts.trailers, o.TrailerAddr)
			case grpc.PeerCallOption:
				observedOpts.peer = append(observedOpts.peer, o.PeerAddr)
			case grpc.PerRPCCredsCallOption:
				observedOpts.creds = append(observedOpts.creds, o.Creds)
			case grpc.FailFastCallOption:
				observedOpts.failFast = append(observedOpts.failFast, o.FailFast)
			case grpc.MaxRecvMsgSizeCallOption:
				observedOpts.maxRecvSize = append(observedOpts.maxRecvSize, o.MaxRecvMsgSize)
			case grpc.MaxSendMsgSizeCallOption:
				observedOpts.maxSendSize = append(observedOpts.maxSendSize, o.MaxSendMsgSize)
			case grpc.CompressorCallOption:
				observedOpts.compressor = append(observedOpts.compressor, o.CompressorType)
			case grpc.ContentSubtypeCallOption:
				observedOpts.subtype = append(observedOpts.subtype, o.ContentSubtype)
			}
		}
	}

	te.unaryClientInt = func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		populateOpts(opts)
		return nil
	}
	te.streamClientInt = func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		populateOpts(opts)
		return nil, nil
	}

	defaults := []grpc.CallOption{
		grpc.FailFast(false),
		grpc.MaxCallRecvMsgSize(1010),
	}
	tc := testpb.NewTestServiceClient(te.clientConn(grpc.WithDefaultCallOptions(defaults...)))

	var headers metadata.MD
	var trailers metadata.MD
	var pr peer.Peer
	tc.UnaryCall(context.Background(), &testpb.SimpleRequest{},
		grpc.MaxCallRecvMsgSize(100),
		grpc.MaxCallSendMsgSize(200),
		grpc.PerRPCCredentials(testPerRPCCredentials{}),
		grpc.Header(&headers),
		grpc.Trailer(&trailers),
		grpc.Peer(&pr))
	expected := observedOptions{
		failFast:    []bool{false},
		maxRecvSize: []int{1010, 100},
		maxSendSize: []int{200},
		creds:       []credentials.PerRPCCredentials{testPerRPCCredentials{}},
		headers:     []*metadata.MD{&headers},
		trailers:    []*metadata.MD{&trailers},
		peer:        []*peer.Peer{&pr},
	}

	if !reflect.DeepEqual(expected, observedOpts) {
		t.Errorf("unary call did not observe expected options: expected %#v, got %#v", expected, observedOpts)
	}

	observedOpts = observedOptions{} // reset

	tc.StreamingInputCall(context.Background(),
		grpc.FailFast(true),
		grpc.MaxCallSendMsgSize(2020),
		grpc.UseCompressor("comp-type"),
		grpc.CallContentSubtype("json"))
	expected = observedOptions{
		failFast:    []bool{false, true},
		maxRecvSize: []int{1010},
		maxSendSize: []int{2020},
		compressor:  []string{"comp-type"},
		subtype:     []string{"json"},
	}

	if !reflect.DeepEqual(expected, observedOpts) {
		t.Errorf("streaming call did not observe expected options: expected %#v, got %#v", expected, observedOpts)
	}
}

func TestCompressorRegister(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testCompressorRegister(t, e)
	}
}

func testCompressorRegister(t *testing.T, e env) {
	te := newTest(t, e)
	te.clientCompression = false
	te.serverCompression = false
	te.clientUseCompression = true

	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	// Unary call
	const argSize = 271828
	const respSize = 314159
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("something", "something"))
	if _, err := tc.UnaryCall(ctx, req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	// Streaming RPC
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: 31415,
		},
	}
	payload, err = newPayload(testpb.PayloadType_COMPRESSABLE, int32(31415))
	if err != nil {
		t.Fatal(err)
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE,
		ResponseParameters: respParam,
		Payload:            payload,
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}
}

func TestServeExitsWhenListenerClosed(t *testing.T) {
	defer leakcheck.Check(t)

	ss := &stubServer{
		emptyCall: func(context.Context, *testpb.Empty) (*testpb.Empty, error) {
			return &testpb.Empty{}, nil
		},
	}

	s := grpc.NewServer()
	testpb.RegisterTestServiceServer(s, ss)

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	done := make(chan struct{})
	go func() {
		s.Serve(lis)
		close(done)
	}()

	cc, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer cc.Close()
	c := testpb.NewTestServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.EmptyCall(ctx, &testpb.Empty{}); err != nil {
		t.Fatalf("Failed to send test RPC to server: %v", err)
	}

	if err := lis.Close(); err != nil {
		t.Fatalf("Failed to close listener: %v", err)
	}
	const timeout = 5 * time.Second
	timer := time.NewTimer(timeout)
	select {
	case <-done:
		return
	case <-timer.C:
		t.Fatalf("Serve did not return after %v", timeout)
	}
}

// Service handler returns status with invalid utf8 message.
func TestStatusInvalidUTF8Message(t *testing.T) {
	defer leakcheck.Check(t)

	var (
		origMsg = string([]byte{0xff, 0xfe, 0xfd})
		wantMsg = "���"
	)

	ss := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			return nil, status.Errorf(codes.Internal, origMsg)
		},
	}
	if err := ss.Start(nil); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer ss.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := ss.client.EmptyCall(ctx, &testpb.Empty{}); status.Convert(err).Message() != wantMsg {
		t.Fatalf("ss.client.EmptyCall(_, _) = _, %v (msg %q); want _, err with msg %q", err, status.Convert(err).Message(), wantMsg)
	}
}

// Service handler returns status with details and invalid utf8 message. Proto
// will fail to marshal the status because of the invalid utf8 message. Details
// will be dropped when sending.
func TestStatusInvalidUTF8Details(t *testing.T) {
	defer leakcheck.Check(t)

	var (
		origMsg = string([]byte{0xff, 0xfe, 0xfd})
		wantMsg = "���"
	)

	ss := &stubServer{
		emptyCall: func(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
			st := status.New(codes.Internal, origMsg)
			st, err := st.WithDetails(&testpb.Empty{})
			if err != nil {
				return nil, err
			}
			return nil, st.Err()
		},
	}
	if err := ss.Start(nil); err != nil {
		t.Fatalf("Error starting endpoint server: %v", err)
	}
	defer ss.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := ss.client.EmptyCall(ctx, &testpb.Empty{})
	st := status.Convert(err)
	if st.Message() != wantMsg {
		t.Fatalf("ss.client.EmptyCall(_, _) = _, %v (msg %q); want _, err with msg %q", err, st.Message(), wantMsg)
	}
	if len(st.Details()) != 0 {
		// Details should be dropped on the server side.
		t.Fatalf("RPC status contain details: %v, want no details", st.Details())
	}
}

func TestClientDoesntDeadlockWhileWritingErrornousLargeMessages(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		if e.httpHandler {
			continue
		}
		testClientDoesntDeadlockWhileWritingErrornousLargeMessages(t, e)
	}
}

func testClientDoesntDeadlockWhileWritingErrornousLargeMessages(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	smallSize := 1024
	te.maxServerReceiveMsgSize = &smallSize
	te.startServer(&testServer{security: e.security})
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, 1048576)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		Payload:      payload,
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
				defer cancel()
				if _, err := tc.UnaryCall(ctx, req); status.Code(err) != codes.ResourceExhausted {
					t.Errorf("TestService/UnaryCall(_,_) = _. %v, want code: %s", err, codes.ResourceExhausted)
					return
				}
			}
		}()
	}
	wg.Wait()
}

const clientAlwaysFailCredErrorMsg = "clientAlwaysFailCred always fails"

var errClientAlwaysFailCred = errors.New(clientAlwaysFailCredErrorMsg)

type clientAlwaysFailCred struct{}

func (c clientAlwaysFailCred) ClientHandshake(ctx context.Context, addr string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errClientAlwaysFailCred
}
func (c clientAlwaysFailCred) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}
func (c clientAlwaysFailCred) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{}
}
func (c clientAlwaysFailCred) Clone() credentials.TransportCredentials {
	return nil
}
func (c clientAlwaysFailCred) OverrideServerName(s string) error {
	return nil
}

func TestFailFastRPCErrorOnBadCertificates(t *testing.T) {
	te := newTest(t, env{name: "bad-cred", network: "tcp", security: "clientAlwaysFailCred", balancer: "round_robin"})
	te.startServer(&testServer{security: te.e.security})
	defer te.tearDown()

	opts := []grpc.DialOption{grpc.WithTransportCredentials(clientAlwaysFailCred{})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, te.srvAddr, opts...)
	if err != nil {
		t.Fatalf("Dial(_) = %v, want %v", err, nil)
	}
	defer cc.Close()

	tc := testpb.NewTestServiceClient(cc)
	for i := 0; i < 1000; i++ {
		// This loop runs for at most 1 second. The first several RPCs will fail
		// with Unavailable because the connection hasn't started. When the
		// first connection failed with creds error, the next RPC should also
		// fail with the expected error.
		if _, err = tc.EmptyCall(context.Background(), &testpb.Empty{}); strings.Contains(err.Error(), clientAlwaysFailCredErrorMsg) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	te.t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want err.Error() contains %q", err, clientAlwaysFailCredErrorMsg)
}

func TestRPCTimeout(t *testing.T) {
	defer leakcheck.Check(t)
	for _, e := range listTestEnv() {
		testRPCTimeout(t, e)
	}
}

func testRPCTimeout(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer(&testServer{security: e.security, unaryCallSleepTime: 500 * time.Millisecond})
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE,
		ResponseSize: respSize,
		Payload:      payload,
	}
	for i := -1; i <= 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(i)*time.Millisecond)
		if _, err := tc.UnaryCall(ctx, req); status.Code(err) != codes.DeadlineExceeded {
			t.Fatalf("TestService/UnaryCallv(_, _) = _, %v; want <nil>, error code: %s", err, codes.DeadlineExceeded)
		}
		cancel()
	}
}
