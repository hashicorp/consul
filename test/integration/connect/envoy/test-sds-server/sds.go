// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	xds "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/logging"
)

const (
	sdsTypeURI = "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret"
)

func main() {
	log := hclog.Default()
	log.SetLevel(hclog.Trace)

	if err := run(log); err != nil {
		log.Error("failed to run SDS server", "err", err)
		os.Exit(1)
	}
}

func run(log hclog.Logger) error {
	cache := cache.NewLinearCache(sdsTypeURI)

	addr := "0.0.0.0:1234"
	if a := os.Getenv("SDS_BIND_ADDR"); a != "" {
		addr = a
	}
	certPath := "certs"
	if p := os.Getenv("SDS_CERT_PATH"); p != "" {
		certPath = p
	}

	if err := loadCertsFromPath(cache, log, certPath); err != nil {
		return err
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	log.Info("==> SDS listening", "addr", addr)

	callbacks := makeLoggerCallbacks(log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	xdsServer := xds.NewServer(ctx, cache, callbacks)
	grpcServer := grpc.NewServer()
	grpclog.SetLoggerV2(logging.NewGRPCLogger("DEBUG", log))

	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, xdsServer)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		grpcServer.Stop()
		cancel()
	}()

	if err := grpcServer.Serve(l); err != nil {
		return err
	}

	return nil
}

func loadCertsFromPath(cache *cache.LinearCache, log hclog.Logger, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".crt") {
			continue
		}

		certName := strings.TrimSuffix(entry.Name(), ".crt")
		cert, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		keyFile := certName + ".key"
		key, err := os.ReadFile(filepath.Join(dir, keyFile))
		if err != nil {
			return err
		}
		var res tls.Secret
		res.Name = certName
		res.Type = &tls.Secret_TlsCertificate{
			TlsCertificate: &tls.TlsCertificate{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{
						InlineBytes: cert,
					},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{
						InlineBytes: key,
					},
				},
			},
		}

		if err := cache.UpdateResource(certName, types.Resource(&res)); err != nil {
			return err
		}
		log.Info("Loaded cert from file", "name", certName)
	}
	return nil
}

func makeLoggerCallbacks(log hclog.Logger) *xds.CallbackFuncs {
	return &xds.CallbackFuncs{

		StreamOpenFunc: func(_ context.Context, id int64, addr string) error {
			log.Trace("gRPC stream opened", "id", id, "addr", addr)
			return nil
		},
		StreamClosedFunc: func(id int64, _ *core.Node) {
			log.Trace("gRPC stream closed", "id", id)
		},
		StreamRequestFunc: func(id int64, req *discovery.DiscoveryRequest) error {
			log.Trace("gRPC stream request", "id", id,
				"node.id", req.Node.Id,
				"req.typeURL", req.TypeUrl,
				"req.version", req.VersionInfo,
			)
			return nil
		},
		StreamResponseFunc: func(_ context.Context, id int64, req *discovery.DiscoveryRequest, resp *discovery.DiscoveryResponse) {
			log.Trace("gRPC stream response", "id", id,
				"resp.typeURL", resp.TypeUrl,
				"resp.version", resp.VersionInfo,
			)
		},
	}
}
