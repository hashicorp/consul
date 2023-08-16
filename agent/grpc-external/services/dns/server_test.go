// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/grpc-external/testutils"
	"github.com/hashicorp/consul/proto-public/pbdns"
)

var txtRR = []string{"Hello world"}

func helloServer(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)

	m.Extra = make([]dns.RR, 1)
	m.Extra[0] = &dns.TXT{
		Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0},
		Txt: txtRR,
	}
	w.WriteMsg(m)
}

func testClient(t *testing.T, server *Server) pbdns.DNSServiceClient {
	t.Helper()

	addr := testutils.RunTestServer(t, server)

	//nolint:staticcheck
	conn, err := grpc.DialContext(context.Background(), addr.String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return pbdns.NewDNSServiceClient(conn)
}

type DNSTestSuite struct {
	suite.Suite
}

func TestDNS_suite(t *testing.T) {
	suite.Run(t, new(DNSTestSuite))
}

func (s *DNSTestSuite) TestProxy_Success() {
	mux := dns.NewServeMux()
	mux.Handle(".", dns.HandlerFunc(helloServer))
	server := NewServer(Config{
		Logger:      hclog.Default(),
		DNSServeMux: mux,
		LocalAddr: LocalAddr{
			net.IPv4(127, 0, 0, 1),
			0,
		},
	})

	client := testClient(s.T(), server)

	testCases := map[string]struct {
		question    string
		clientQuery func(qR *pbdns.QueryRequest)
		expectedErr error
	}{

		"happy path udp": {
			question: "abc.com.",
			clientQuery: func(qR *pbdns.QueryRequest) {
				qR.Protocol = pbdns.Protocol_PROTOCOL_UDP
			},
		},
		"happy path tcp": {
			question: "abc.com.",
			clientQuery: func(qR *pbdns.QueryRequest) {
				qR.Protocol = pbdns.Protocol_PROTOCOL_TCP
			},
		},
		"No protocol set": {
			question:    "abc.com.",
			clientQuery: func(qR *pbdns.QueryRequest) {},
			expectedErr: errors.New("error protocol type not set: PROTOCOL_UNSET_UNSPECIFIED"),
		},
		"Invalid question": {
			question: "notvalid",
			clientQuery: func(qR *pbdns.QueryRequest) {
				qR.Protocol = pbdns.Protocol_PROTOCOL_UDP
			},
			expectedErr: errors.New("failure decoding dns request"),
		},
	}

	for name, tc := range testCases {
		s.Run(name, func() {
			req := dns.Msg{}
			req.SetQuestion(tc.question, dns.TypeA)

			bytes, _ := req.Pack()

			clientReq := &pbdns.QueryRequest{Msg: bytes}
			tc.clientQuery(clientReq)
			clientResp, err := client.Query(context.Background(), clientReq)
			if tc.expectedErr != nil {
				s.Require().Error(err, "no errror calling gRPC endpoint")
				s.Require().ErrorContains(err, tc.expectedErr.Error())
			} else {
				s.Require().NoError(err, "error calling gRPC endpoint")

				resp := clientResp.GetMsg()
				var dnsResp dns.Msg

				err = dnsResp.Unpack(resp)
				s.Require().NoError(err, "error unpacking dns response")
				rr := dnsResp.Extra[0].(*dns.TXT)
				s.Require().EqualValues(rr.Txt, txtRR)
			}
		})
	}
}
