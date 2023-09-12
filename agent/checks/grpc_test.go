package checks

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/go-hclog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	port         int
	server       string
	svcHealthy   string
	svcUnhealthy string
	svcMissing   string
)

func startServer() (*health.Server, *grpc.Server) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	server := health.NewServer()
	hv1.RegisterHealthServer(grpcServer, server)
	go grpcServer.Serve(listener)
	return server, grpcServer
}

func init() {
	flag.IntVar(&port, "grpc-stub-port", 54321, "port for the gRPC stub server")
}

func TestMain(m *testing.M) {
	flag.Parse()

	healthy := "healthy"
	unhealthy := "unhealthy"
	missing := "missing"

	srv, grpcStubApp := startServer()
	srv.SetServingStatus(healthy, hv1.HealthCheckResponse_SERVING)
	srv.SetServingStatus(unhealthy, hv1.HealthCheckResponse_NOT_SERVING)

	server = fmt.Sprintf("%s:%d", "localhost", port)
	svcHealthy = fmt.Sprintf("%s/%s", server, healthy)
	svcUnhealthy = fmt.Sprintf("%s/%s", server, unhealthy)
	svcMissing = fmt.Sprintf("%s/%s", server, missing)

	result := 1
	defer func() {
		grpcStubApp.Stop()
		os.Exit(result)
	}()

	result = m.Run()
}

func TestCheck(t *testing.T) {
	type args struct {
		target    string
		timeout   time.Duration
		tlsConfig *tls.Config
	}
	tests := []struct {
		name    string
		args    args
		healthy bool
	}{
		// successes
		{"should pass for healthy server", args{server, time.Second, nil}, true},
		{"should pass for healthy service", args{svcHealthy, time.Second, nil}, true},

		// failures
		{"should fail for unhealthy service", args{svcUnhealthy, time.Second, nil}, false},
		{"should fail for missing service", args{svcMissing, time.Second, nil}, false},
		{"should timeout for healthy service", args{server, time.Nanosecond, nil}, false},
		{"should fail if probe is secure, but server is not", args{server, time.Second, &tls.Config{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := NewGrpcHealthProbe(tt.args.target, tt.args.timeout, tt.args.tlsConfig)
			actualError := probe.Check(tt.args.target)
			actuallyHealthy := actualError == nil
			if tt.healthy != actuallyHealthy {
				t.Errorf("FAIL: %s. Expected healthy %t, but err == %v", tt.name, tt.healthy, actualError)
			}
		})
	}
}

func TestGRPC_Proxied(t *testing.T) {
	t.Parallel()

	notif := mock.NewNotify()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   uniqueID(),
		Output: ioutil.Discard,
	})

	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckGRPC{
		CheckID:       cid,
		GRPC:          "",
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		ProxyGRPC:     server,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	// If ProxyGRPC is set, check() reqs should go to that address
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestGRPC_NotProxied(t *testing.T) {
	t.Parallel()

	notif := mock.NewNotify()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   uniqueID(),
		Output: ioutil.Discard,
	})

	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckGRPC{
		CheckID:       cid,
		GRPC:          server,
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		ProxyGRPC:     "",
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	// If ProxyGRPC is not set, check() reqs should go to check.GRPC
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}
