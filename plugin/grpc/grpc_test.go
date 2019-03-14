package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/coredns/coredns/pb"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestGRPC(t *testing.T) {
	m := &dns.Msg{}
	msg, err := m.Pack()
	if err != nil {
		t.Fatalf("Error packing response: %s", err.Error())
	}
	dnsPacket := &pb.DnsPacket{Msg: msg}
	tests := map[string]struct {
		proxies []*Proxy
		wantErr bool
	}{
		"single_proxy_ok": {
			proxies: []*Proxy{
				{client: &testServiceClient{dnsPacket: dnsPacket, err: nil}},
			},
			wantErr: false,
		},
		"multiple_proxies_ok": {
			proxies: []*Proxy{
				{client: &testServiceClient{dnsPacket: dnsPacket, err: nil}},
				{client: &testServiceClient{dnsPacket: dnsPacket, err: nil}},
				{client: &testServiceClient{dnsPacket: dnsPacket, err: nil}},
			},
			wantErr: false,
		},
		"single_proxy_ko": {
			proxies: []*Proxy{
				{client: &testServiceClient{dnsPacket: nil, err: errors.New("")}},
			},
			wantErr: true,
		},
		"multiple_proxies_one_ko": {
			proxies: []*Proxy{
				{client: &testServiceClient{dnsPacket: dnsPacket, err: nil}},
				{client: &testServiceClient{dnsPacket: nil, err: errors.New("")}},
				{client: &testServiceClient{dnsPacket: dnsPacket, err: nil}},
			},
			wantErr: false,
		},
		"multiple_proxies_ko": {
			proxies: []*Proxy{
				{client: &testServiceClient{dnsPacket: nil, err: errors.New("")}},
				{client: &testServiceClient{dnsPacket: nil, err: errors.New("")}},
				{client: &testServiceClient{dnsPacket: nil, err: errors.New("")}},
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			g := newGRPC()
			g.from = "."
			g.proxies = tt.proxies
			rec := dnstest.NewRecorder(&test.ResponseWriter{})
			if _, err := g.ServeDNS(context.TODO(), rec, m); err != nil && !tt.wantErr {
				t.Fatal("Expected to receive reply, but didn't")
			}
		})
	}
}
