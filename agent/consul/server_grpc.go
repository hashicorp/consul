// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/fullstorydev/grpchan/inprocgrpc"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	aclgrpc "github.com/hashicorp/consul/agent/grpc-external/services/acl"
	"github.com/hashicorp/consul/agent/grpc-external/services/configentry"
	"github.com/hashicorp/consul/agent/grpc-external/services/connectca"
	"github.com/hashicorp/consul/agent/grpc-external/services/dataplane"
	"github.com/hashicorp/consul/agent/grpc-external/services/peerstream"
	resourcegrpc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/agent/grpc-external/services/serverdiscovery"
	agentgrpc "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/services/subscribe"
	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/agent/rpc/operator"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

func (s *Server) setupGRPCInterfaces(config *Config, deps Deps) error {
	// A server has 5 different gRPC interfaces
	//
	// * External - This is the main public gRPC network listener. This
	//      is an actual *grpc.Server that we have listening on both the
	//      grpc and grpc_tls ports. Generally this interface will not be
	//      used by the server itself. All services which are intended
	//      to be public APIs must be registered to this interface. This
	//      interface is created outside of the server in the agent code
	//      and then passed to the NewServer constructor. Some services
	//      like xDS and DNS get registered outside of the server code.
	//
	// * Internal / Multiplexed - Our internal_rpc port uses yamux and
	//      various byte prefixes to multiplex different protocols over
	//      the single connection. One of the multiplexed protocols is
	//      gRPC. gRPC in this fashion works using a custom net.Listener
	//      implementation that receives net.Conns to be handled through
	//      a channel. When a new yamux session is opened which produces
	//      a yamux conn (which implements the net.Conn interface), the
	//      connection is then sent to the custom listener. Then the
	//      standard grpc.Server.Serve method can accept the conn from
	//      the listener and operate on it like any other standard conn.
	//      Historically, the external gRPC interface was optional and
	//      so all services which needed leader or DC forwarding had to
	//      be exposed on this interface in order to guarantee they
	//      would be available. In the future, an external gRPC interface
	//      likely will be required and the services which need registering
	//      to the multiplexed listener will be greatly reduced. In the
	//      very long term we want to get rid of this internal multiplexed
	//      port/listener and instead have all component communications use
	//      gRPC natively. For now though, if your service will need to
	//      RECEIVE forwarded requests then it must be registered to this
	//      interface.
	//
	// * In-Process - For routines running on the server we don't want them
	//      to require network i/o as that will incur a lot of unnecessary
	//      overhead. To avoid that we are utilizing the `grpchan` library
	//      (github.com/fullstorydev/grpchan) and its `inprocgrpc` package.
	//      The library provides the `inprocgrpc.Channel` which implements
	//      both the `grpc.ServiceRegistrar` and `grpc.ClientConnInterface`
	//      interfaces. Services get registered to the `Channel` and then
	//      gRPC service clients can be created with the `Channel` used
	//      for the backing `ClientConn`. When a client then uses the
	//      `Invoke` or `NewStream` methods on the `Channel`, the `Channel`
	//      will lookup in its registry of services to find the service's
	//      server implementation and then have the standard
	//      grpc.MethodDesc.Handler function handle the request. We use
	//      a few variants of the in-process gRPC Channel. For now all
	//      these channels are created and managed in server code but we
	//      may need to move these into the higher level agent setup.
	//
	//      * Insecure + Unsafe - The insecure + unsafe gRPC Channel has
	//           services registered to it that wont do typical ACL
	//           resolution. Instead when the service resolves ACL tokens
	//           a resolver is used which always grants unrestricted
	//           privileges. Additionally, this "unsafe" variant DOES
	//           NOT clone resources as they pass through the channel. Care
	//           Must be taken to note mutate the data passed through the
	//           Channel or else we could easily cause data race related
	//           or consistency bugs.
	//
	//      * Insecure + Safe - Similar to the Insecure + Unsafe variant,
	//           ACL resolution always provides an authorizer with unrestricted
	//           privileges. However, this interface is concurrency/memory safe
	//           in that protobuf messages passing through the interface are
	//           cloned so that the client is free to mutate those messages
	//           once the request is complete. All services registered to the
	//           Unsafe variant should also be registered to this interface.
	//
	//      * Secure + Safe - This Channel will do typical ACL resolution from
	//           tokens and will clone protobuf messages that pass through. This
	//           interface will be useful for something like the HTTP server that
	//           is crafting the gRPC requests from a user request and wants to
	//           assume no implicit privileges by the nature of running on the
	//           server. All services registered to the insecure variants should
	//           also be registered to this interface. Additionally other services
	//           that correspond to user requests should also be registered to this
	//           interface.
	//
	//      Currently there is not a need for a Secure + Unsafe variant. We could
	//      add it if needed in the future.

	recoveryOpts := agentmiddleware.PanicHandlerMiddlewareOpts(s.loggers.Named(logging.GRPCAPI))

	inprocLabels := []metrics.Label{{
		Name:  "server_type",
		Value: "in-process",
	}}

	statsHandler := agentmiddleware.NewStatsHandler(metrics.Default(), inprocLabels)

	// TODO(inproc-grpc) - figure out what to do with rate limiting inproc grpc. If we
	// want to rate limit in-process clients then we are going to need a unary interceptor
	// to do that. Another idea would be to create rate limited clients which can be given
	// to controllers or other internal code so that the whole Channel isn't limited but
	// rather individual consumers of that channel.

	// Build the Insecure + Unsafe gRPC Channel
	s.insecureUnsafeGRPCChan = new(inprocgrpc.Channel).
		// Bypass the in-process gRPCs cloning functionality by providing
		// a Cloner implementation which doesn't actually clone the data.
		// Note that this is only done for the Unsafe gRPC Channel and
		// all the Safe variants will utilize the default cloning
		// functionality.
		WithCloner(inprocgrpc.CloneFunc(func(in any) (any, error) {
			return in, nil
		})).
		WithServerUnaryInterceptor(middleware.ChainUnaryServer(
			recovery.UnaryServerInterceptor(recoveryOpts...),
			statsHandler.Intercept,
		)).
		WithServerStreamInterceptor(middleware.ChainStreamServer(
			recovery.StreamServerInterceptor(recoveryOpts...),
			agentmiddleware.NewActiveStreamCounter(metrics.Default(), inprocLabels).Intercept,
		))

	// Build the Insecure + Safe gRPC Channel
	s.insecureSafeGRPCChan = new(inprocgrpc.Channel).
		WithServerUnaryInterceptor(middleware.ChainUnaryServer(
			recovery.UnaryServerInterceptor(recoveryOpts...),
			statsHandler.Intercept,
		)).
		WithServerStreamInterceptor(middleware.ChainStreamServer(
			recovery.StreamServerInterceptor(recoveryOpts...),
			agentmiddleware.NewActiveStreamCounter(metrics.Default(), inprocLabels).Intercept,
		))

	// Build the Secure + Safe gRPC Channel
	s.secureSafeGRPCChan = new(inprocgrpc.Channel).
		WithServerUnaryInterceptor(middleware.ChainUnaryServer(
			recovery.UnaryServerInterceptor(recoveryOpts...),
			statsHandler.Intercept,
		)).
		WithServerStreamInterceptor(middleware.ChainStreamServer(
			recovery.StreamServerInterceptor(recoveryOpts...),
			agentmiddleware.NewActiveStreamCounter(metrics.Default(), inprocLabels).Intercept,
		))

	// create the internal multiplexed gRPC interface
	s.internalGRPCHandler = agentgrpc.NewHandler(deps.Logger, config.RPCAddr, nil, s.incomingRPCLimiter)

	return nil
}

func (s *Server) setupGRPCServices(config *Config, deps Deps) error {
	// Register the resource service with the in-process registrars WITHOUT AUTHORIZATION
	err := s.registerResourceServiceServer(
		deps.Registry,
		resolver.DANGER_NO_AUTH{},
		s.insecureUnsafeGRPCChan,
		s.insecureSafeGRPCChan)
	if err != nil {
		return err
	}

	// Register the resource service with all other registrars other
	// than the internal/multiplexed interface. Currently there is
	// no need to forward resource service RPCs and therefore the
	// service doesn't need to be available on that interface.
	err = s.registerResourceServiceServer(
		deps.Registry,
		s.ACLResolver,
		s.secureSafeGRPCChan,
		s.internalGRPCHandler,
		s.externalGRPCServer,
	)
	if err != nil {
		return err
	}

	// The ACL grpc services get registered with all "secure" gRPC interfaces
	err = s.registerACLServer(
		s.secureSafeGRPCChan,
		s.externalGRPCServer,
		s.internalGRPCHandler,
	)
	if err != nil {
		return err
	}

	// register the Connect CA service on all "secure" interfaces
	err = s.registerConnectCAServer(
		s.secureSafeGRPCChan,
		s.externalGRPCServer,
		s.internalGRPCHandler,
	)
	if err != nil {
		return err
	}

	// Initializing the peering backend must be done before
	// creating any peering servers. There is other code which
	// calls methods on this and so the backend must be stored
	// on the Server type. In the future we should investigate
	// whether we can not require the backend in that other code.
	s.peeringBackend = NewPeeringBackend(s)

	// register the peering service on the external gRPC server only
	// As this service is only ever accessed externally there is
	// no need to register it on the various in-process Channels
	s.peerStreamServer, err = s.registerPeerStreamServer(
		config,
		s.externalGRPCServer,
		s.internalGRPCHandler,
	)
	if err != nil {
		return err
	}

	// register the peering service on the internal interface only. As
	// the peering gRPC service is a private API its only ever accessed
	// via the internalGRPCHandler with an actual network conn managed
	// by the Agents GRPCConnPool.
	err = s.registerPeeringServer(
		config,
		s.internalGRPCHandler,
	)
	if err != nil {
		return err
	}

	// Register the Operator service on all "secure" interfaces. The
	// operator service is currently only accessed via the
	// internalGRPCHandler but in the future these APIs are likely to
	// become part of our "public" API and so it should be exposed on
	// more interfaces.
	err = s.registerOperatorServer(
		config,
		deps,
		s.internalGRPCHandler,
		s.secureSafeGRPCChan,
		s.externalGRPCServer,
	)
	if err != nil {
		return err
	}

	// register the stream subscription service on the multiplexed internal interface
	// if stream is enabled.
	if config.RPCConfig.EnableStreaming {
		err = s.registerStreamSubscriptionServer(
			deps,
			s.internalGRPCHandler,
		)
		if err != nil {
			return err
		}
	}

	// register the server discovery service on all "secure" interfaces other
	// than the multiplexed internal interface. This service is mainly consumed
	// by the consul-server-connection-manager library which is used by various
	// other system components other than the agent.
	err = s.registerServerDiscoveryServer(
		s.ACLResolver,
		s.secureSafeGRPCChan,
		s.externalGRPCServer,
	)
	if err != nil {
		return err
	}

	// register the server discovery service on the insecure in-process channels.
	// Currently, this is unused but eventually things such as the peering service
	// should be refactored to consume the in-memory service instead of hooking
	// directly into an the event publisher and subscribing to specific events.
	err = s.registerServerDiscoveryServer(
		resolver.DANGER_NO_AUTH{},
		s.insecureUnsafeGRPCChan,
		s.insecureSafeGRPCChan,
	)
	if err != nil {
		return err
	}

	// register the data plane service on the external gRPC server only. This
	// service is only access by dataplanes and at this time there is no need
	// for anything internal in Consul to use the service. If that changes
	// we could register it on the in-process interfaces as well.
	err = s.registerDataplaneServer(
		deps,
		s.externalGRPCServer,
	)
	if err != nil {
		return err
	}

	// register the configEntry service on the internal interface only. As
	// it is only accessed via the internalGRPCHandler with an actual network
	// conn managed  by the Agents GRPCConnPool.
	err = s.registerConfigEntryServer(
		s.internalGRPCHandler,
	)
	if err != nil {
		return err
	}

	// enable grpc server reflection for the external gRPC interface only
	reflection.Register(s.externalGRPCServer)

	return s.setupEnterpriseGRPCServices(config, deps)
}

func (s *Server) registerResourceServiceServer(typeRegistry resource.Registry, resolver resourcegrpc.ACLResolver, registrars ...grpc.ServiceRegistrar) error {
	if s.storageBackend == nil {
		return fmt.Errorf("storage backend cannot be nil")
	}

	var tenancyBridge resourcegrpc.TenancyBridge
	if s.useV2Tenancy {
		tenancyBridge = tenancy.NewV2TenancyBridge().WithClient(
			// This assumes that the resource service will be registered with
			// the insecureUnsafeGRPCChan. We are using the insecure and unsafe
			// channel here because the V2 Tenancy bridge only reads data
			// from the client and does not modify it. Therefore sharing memory
			// with the resource services canonical immutable data is advantageous
			// to prevent wasting CPU time for every resource op to clone things.
			pbresource.NewResourceServiceClient(s.insecureUnsafeGRPCChan),
		)
	} else {
		tenancyBridge = NewV1TenancyBridge(s)
	}

	// Create the Resource Service Server
	srv := resourcegrpc.NewServer(resourcegrpc.Config{
		Registry:      typeRegistry,
		Backend:       s.storageBackend,
		ACLResolver:   resolver,
		Logger:        s.loggers.Named(logging.GRPCAPI).Named(logging.Resource),
		TenancyBridge: tenancyBridge,
		UseV2Tenancy:  s.useV2Tenancy,
	})

	// Register the server to all the desired interfaces
	for _, reg := range registrars {
		pbresource.RegisterResourceServiceServer(reg, srv)
	}
	return nil
}

func (s *Server) registerACLServer(registrars ...grpc.ServiceRegistrar) error {
	srv := aclgrpc.NewServer(aclgrpc.Config{
		ACLsEnabled: s.config.ACLsEnabled,
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		InPrimaryDatacenter: s.InPrimaryDatacenter(),
		LoadAuthMethod: func(methodName string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, aclgrpc.Validator, error) {
			return s.loadAuthMethod(methodName, entMeta)
		},
		LocalTokensEnabled:        s.LocalTokensEnabled,
		Logger:                    s.loggers.Named(logging.GRPCAPI).Named(logging.ACL),
		NewLogin:                  func() aclgrpc.Login { return s.aclLogin() },
		NewTokenWriter:            func() aclgrpc.TokenWriter { return s.aclTokenWriter() },
		PrimaryDatacenter:         s.config.PrimaryDatacenter,
		ValidateEnterpriseRequest: s.validateEnterpriseRequest,
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}

func (s *Server) registerPeerStreamServer(config *Config, registrars ...grpc.ServiceRegistrar) (*peerstream.Server, error) {
	if s.peeringBackend == nil {
		panic("peeringBackend is required during construction")
	}

	srv := peerstream.NewServer(peerstream.Config{
		Backend:        s.peeringBackend,
		GetStore:       func() peerstream.StateStore { return s.FSM().State() },
		Logger:         s.loggers.Named(logging.GRPCAPI).Named(logging.PeerStream),
		ACLResolver:    s.ACLResolver,
		Datacenter:     s.config.Datacenter,
		ConnectEnabled: s.config.ConnectEnabled,
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			// Only forward the request if the dc in the request matches the server's datacenter.
			if info.RequestDatacenter() != "" && info.RequestDatacenter() != config.Datacenter {
				return false, fmt.Errorf("requests to generate peering tokens cannot be forwarded to remote datacenters")
			}
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return srv, nil
}

func (s *Server) registerPeeringServer(config *Config, registrars ...grpc.ServiceRegistrar) error {
	if s.peeringBackend == nil {
		panic("peeringBackend is required during construction")
	}

	if s.peerStreamServer == nil {
		panic("the peer stream server must be configured before the peering server")
	}

	srv := peering.NewServer(peering.Config{
		Backend: s.peeringBackend,
		Tracker: s.peerStreamServer.Tracker,
		Logger:  s.loggers.Named(logging.GRPCAPI).Named(logging.Peering),
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			// Only forward the request if the dc in the request matches the server's datacenter.
			if info.RequestDatacenter() != "" && info.RequestDatacenter() != config.Datacenter {
				return false, fmt.Errorf("requests to generate peering tokens cannot be forwarded to remote datacenters")
			}
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		Datacenter:     config.Datacenter,
		ConnectEnabled: config.ConnectEnabled,
		PeeringEnabled: config.PeeringEnabled,
		Locality:       config.Locality,
		FSMServer:      s,
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}

func (s *Server) registerOperatorServer(config *Config, deps Deps, registrars ...grpc.ServiceRegistrar) error {
	srv := operator.NewServer(operator.Config{
		Backend: NewOperatorBackend(s),
		Logger:  deps.Logger.Named("grpc-api.operator"),
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			// Only forward the request if the dc in the request matches the server's datacenter.
			if info.RequestDatacenter() != "" && info.RequestDatacenter() != config.Datacenter {
				return false, fmt.Errorf("requests to transfer leader cannot be forwarded to remote datacenters")
			}
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		Datacenter: config.Datacenter,
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}

func (s *Server) registerStreamSubscriptionServer(deps Deps, registrars ...grpc.ServiceRegistrar) error {
	srv := subscribe.NewServer(
		&subscribeBackend{srv: s, connPool: deps.GRPCConnPool},
		s.loggers.Named(logging.GRPCAPI).Named("subscription"),
	)

	for _, reg := range registrars {
		pbsubscribe.RegisterStateChangeSubscriptionServer(reg, srv)
	}

	return nil
}

func (s *Server) registerConnectCAServer(registrars ...grpc.ServiceRegistrar) error {
	srv := connectca.NewServer(connectca.Config{
		Publisher:   s.publisher,
		GetStore:    func() connectca.StateStore { return s.FSM().State() },
		Logger:      s.loggers.Named(logging.GRPCAPI).Named(logging.ConnectCA),
		ACLResolver: s.ACLResolver,
		CAManager:   s.caManager,
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		ConnectEnabled: s.config.ConnectEnabled,
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}

func (s *Server) registerDataplaneServer(deps Deps, registrars ...grpc.ServiceRegistrar) error {
	srv := dataplane.NewServer(dataplane.Config{
		GetStore:          func() dataplane.StateStore { return s.FSM().State() },
		Logger:            s.loggers.Named(logging.GRPCAPI).Named(logging.Dataplane),
		ACLResolver:       s.ACLResolver,
		Datacenter:        s.config.Datacenter,
		EnableV2:          stringslice.Contains(deps.Experiments, CatalogResourceExperimentName),
		ResourceAPIClient: pbresource.NewResourceServiceClient(s.insecureSafeGRPCChan),
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}

func (s *Server) registerServerDiscoveryServer(resolver serverdiscovery.ACLResolver, registrars ...grpc.ServiceRegistrar) error {
	srv := serverdiscovery.NewServer(serverdiscovery.Config{
		Publisher:   s.publisher,
		ACLResolver: resolver,
		Logger:      s.loggers.Named(logging.GRPCAPI).Named(logging.ServerDiscovery),
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}

func (s *Server) registerConfigEntryServer(registrars ...grpc.ServiceRegistrar) error {

	srv := configentry.NewServer(configentry.Config{
		Backend: NewConfigEntryBackend(s),
		Logger:  s.loggers.Named(logging.GRPCAPI).Named(logging.ConfigEntry),
		ForwardRPC: func(info structs.RPCInfo, fn func(*grpc.ClientConn) error) (bool, error) {
			return s.ForwardGRPC(s.grpcConnPool, info, fn)
		},
		FSMServer: s,
	})

	for _, reg := range registrars {
		srv.Register(reg)
	}

	return nil
}
