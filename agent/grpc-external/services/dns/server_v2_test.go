// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"errors"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"

	agentdns "github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/proto-public/pbdns"
)

func basicResponse() *dns.Msg {
	return &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Opcode:        dns.OpcodeQuery,
			Response:      true,
			Authoritative: true,
		},
		Compress: true,
		Question: []dns.Question{
			{
				Name:   "abc.com.",
				Qtype:  dns.TypeANY,
				Qclass: dns.ClassINET,
			},
		},
		Extra: []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "abc.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				Txt: txtRR,
			},
		},
	}
}

func (s *DNSTestSuite) TestProxy_V2Success() {

	testCases := map[string]struct {
		question        string
		configureRouter func(router *agentdns.MockDNSRouter)
		clientQuery     func(qR *pbdns.QueryRequest)
		expectedErr     error
	}{

		"happy path udp": {
			question: "abc.com.",
			configureRouter: func(router *agentdns.MockDNSRouter) {
				router.On("HandleRequest", mock.Anything, mock.Anything, mock.Anything).
					Return(basicResponse(), nil)
			},
			clientQuery: func(qR *pbdns.QueryRequest) {
				qR.Protocol = pbdns.Protocol_PROTOCOL_UDP
			},
		},
		"happy path tcp": {
			question: "abc.com.",
			configureRouter: func(router *agentdns.MockDNSRouter) {
				router.On("HandleRequest", mock.Anything, mock.Anything, mock.Anything).
					Return(basicResponse(), nil)
			},
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
			router := agentdns.NewMockDNSRouter(s.T())

			if tc.configureRouter != nil {
				tc.configureRouter(router)
			}

			server := NewServerV2(ConfigV2{
				Logger:    hclog.Default(),
				DNSRouter: router,
				TokenFunc: func() string { return "" },
			})

			client := testClient(s.T(), server)

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
