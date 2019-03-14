package grpc

import (
	"context"
	"crypto/tls"
	"strconv"
	"time"

	"github.com/coredns/coredns/pb"

	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// Proxy defines an upstream host.
type Proxy struct {
	addr string

	// connection
	client   pb.DnsServiceClient
	dialOpts []grpc.DialOption
}

// newProxy returns a new proxy.
func newProxy(addr string, tlsConfig *tls.Config) (*Proxy, error) {
	p := &Proxy{
		addr: addr,
	}

	if tlsConfig != nil {
		p.dialOpts = append(p.dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		p.dialOpts = append(p.dialOpts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(p.addr, p.dialOpts...)
	if err != nil {
		return nil, err
	}
	p.client = pb.NewDnsServiceClient(conn)

	return p, nil
}

// query sends the request and waits for a response.
func (p *Proxy) query(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	start := time.Now()

	msg, err := req.Pack()
	if err != nil {
		return nil, err
	}

	reply, err := p.client.Query(ctx, &pb.DnsPacket{Msg: msg})
	if err != nil {
		// if not found message, return empty message with NXDomain code
		if status.Code(err) == codes.NotFound {
			m := new(dns.Msg).SetRcode(req, dns.RcodeNameError)
			return m, nil
		}
		return nil, err
	}
	ret := new(dns.Msg)
	if err := ret.Unpack(reply.Msg); err != nil {
		return nil, err
	}

	rc, ok := dns.RcodeToString[ret.Rcode]
	if !ok {
		rc = strconv.Itoa(ret.Rcode)
	}

	RequestCount.WithLabelValues(p.addr).Add(1)
	RcodeCount.WithLabelValues(rc, p.addr).Add(1)
	RequestDuration.WithLabelValues(p.addr).Observe(time.Since(start).Seconds())

	return ret, nil
}
