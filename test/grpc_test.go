package test

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
	"google.golang.org/grpc"

	"github.com/coredns/coredns/pb"
)

func TestGrpc(t *testing.T) {
	corefile := `grpc://.:0 {
		whoami
}
`
	g, _, tcp, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer g.Stop()

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := grpc.DialContext(ctx, tcp, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Expected no error but got: %s", err)
	}
	defer conn.Close()

	client := pb.NewDnsServiceClient(conn)

	m := new(dns.Msg)
	m.SetQuestion("whoami.example.org.", dns.TypeA)
	msg, _ := m.Pack()

	reply, err := client.Query(context.TODO(), &pb.DnsPacket{Msg: msg})
	if err != nil {
		t.Errorf("Expected no error but got: %s", err)
	}

	d := new(dns.Msg)
	err = d.Unpack(reply.Msg)
	if err != nil {
		t.Errorf("Expected no error but got: %s", err)
	}

	if d.Rcode != dns.RcodeSuccess {
		t.Errorf("Expected success but got %d", d.Rcode)
	}

	if len(d.Extra) != 2 {
		t.Errorf("Expected 2 RRs in additional section, but got %d", len(d.Extra))
	}
}
