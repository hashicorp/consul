package proxy

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/coredns/coredns/middleware/proxy/pb"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type grpcClient struct {
	dialOpt  grpc.DialOption
	clients  map[string]pb.DnsServiceClient
	conns    []*grpc.ClientConn
	upstream *staticUpstream
}

func newGrpcClient(tls *tls.Config, u *staticUpstream) *grpcClient {
	g := &grpcClient{upstream: u}

	if tls == nil {
		g.dialOpt = grpc.WithInsecure()
	} else {
		g.dialOpt = grpc.WithTransportCredentials(credentials.NewTLS(tls))
	}
	g.clients = map[string]pb.DnsServiceClient{}

	return g
}

func (g *grpcClient) Exchange(ctx context.Context, addr string, state request.Request) (*dns.Msg, error) {
	msg, err := state.Req.Pack()
	if err != nil {
		return nil, err
	}

	reply, err := g.clients[addr].Query(ctx, &pb.DnsPacket{Msg: msg})
	if err != nil {
		return nil, err
	}
	d := new(dns.Msg)
	err = d.Unpack(reply.Msg)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (g *grpcClient) Protocol() string { return "grpc" }

func (g *grpcClient) OnShutdown(p *Proxy) error {
	for i, conn := range g.conns {
		err := conn.Close()
		if err != nil {
			log.Printf("[WARNING] Error closing connection %d: %s\n", i, err)
		}
	}
	return nil
}

func (g *grpcClient) OnStartup(p *Proxy) error {
	for _, host := range g.upstream.Hosts {
		conn, err := grpc.Dial(host.Name, g.dialOpt)
		if err != nil {
			log.Printf("[WARNING] Skipping gRPC host '%s' due to Dial error: %s\n", host.Name, err)
		} else {
			g.clients[host.Name] = pb.NewDnsServiceClient(conn)
			g.conns = append(g.conns, conn)
		}
	}
	return nil
}
