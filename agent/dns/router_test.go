// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/armon/go-radix"

	"github.com/hashicorp/consul/internal/dnsutil"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
)

// HandleTestCase is a test case for the HandleRequest function.
// Tests for HandleRequest are split into multiple files to make it easier to
// manage and understand the tests.  Other test files are:
// - router_addr_test.go
// - router_ns_test.go
// - router_prepared_query_test.go
// - router_ptr_test.go
// - router_recursor_test.go
// - router_service_test.go
// - router_soa_test.go
// - router_virtual_test.go
// - router_v2_services_test.go
// - router_workload_test.go
type HandleTestCase struct {
	name                         string
	agentConfig                  *config.RuntimeConfig // This will override the default test Router Config
	configureDataFetcher         func(fetcher discovery.CatalogDataFetcher)
	validateAndNormalizeExpected bool
	configureRecursor            func(recursor dnsRecursor)
	mockProcessorError           error
	request                      *dns.Msg
	requestContext               *Context
	remoteAddress                net.Addr
	response                     *dns.Msg
}

func Test_HandleRequest_Validation(t *testing.T) {
	testCases := []HandleTestCase{
		{
			name: "request with empty message",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{},
			},
			validateAndNormalizeExpected: false,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Response:      true,
					Authoritative: false,
					Rcode:         dns.RcodeRefused,
				},
				Compress: false,
				Question: nil,
				Answer:   nil,
				Ns:       nil,
				Extra:    nil,
			},
		},
		// Context Tests
		{
			name: "When a request context is provided, use those field in the query",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			requestContext: &Context{
				Token:            "test-token",
				DefaultNamespace: "test-namespace",
				DefaultPartition: "test-partition",
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := []*discovery.Result{
					{
						Type: discovery.ResultTypeNode,
						Node: &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Tenancy: discovery.ResultTenancy{
							Namespace: "test-namespace",
							Partition: "test-partition",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(result, nil).
					Run(func(args mock.Arguments) {
						ctx := args.Get(0).(discovery.Context)
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "test-token", ctx.Token)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "test-namespace", req.Tenancy.Namespace)
						require.Equal(t, "test-partition", req.Tenancy.Partition)

						require.Equal(t, discovery.LookupTypeService, reqType)
					})
			},
			validateAndNormalizeExpected: true,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "foo.service.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.service.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
		{
			name: "When a request context is provided, values do not override explicit tenancy",
			request: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
				},
				Question: []dns.Question{
					{
						Name:   "foo.service.bar.ns.baz.ap.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
			},
			requestContext: &Context{
				Token:            "test-token",
				DefaultNamespace: "test-namespace",
				DefaultPartition: "test-partition",
			},
			configureDataFetcher: func(fetcher discovery.CatalogDataFetcher) {
				result := []*discovery.Result{
					{
						Type: discovery.ResultTypeNode,
						Node: &discovery.Location{Name: "foo", Address: "1.2.3.4"},
						Tenancy: discovery.ResultTenancy{
							Namespace: "bar",
							Partition: "baz",
						},
					},
				}

				fetcher.(*discovery.MockCatalogDataFetcher).
					On("FetchEndpoints", mock.Anything, mock.Anything, mock.Anything).
					Return(result, nil).
					Run(func(args mock.Arguments) {
						ctx := args.Get(0).(discovery.Context)
						req := args.Get(1).(*discovery.QueryPayload)
						reqType := args.Get(2).(discovery.LookupType)

						require.Equal(t, "test-token", ctx.Token)

						require.Equal(t, "foo", req.Name)
						require.Equal(t, "bar", req.Tenancy.Namespace)
						require.Equal(t, "baz", req.Tenancy.Partition)

						require.Equal(t, discovery.LookupTypeService, reqType)
					})
			},
			validateAndNormalizeExpected: true,
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Response:      true,
					Authoritative: true,
				},
				Compress: true,
				Question: []dns.Question{
					{
						Name:   "foo.service.bar.ns.baz.ap.consul.",
						Qtype:  dns.TypeA,
						Qclass: dns.ClassINET,
					},
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "foo.service.bar.ns.baz.ap.consul.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    123,
						},
						A: net.ParseIP("1.2.3.4"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runHandleTestCases(t, tc)
		})
	}
}

// runHandleTestCases runs the test cases for the HandleRequest function.
func runHandleTestCases(t *testing.T, tc HandleTestCase) {
	cdf := discovery.NewMockCatalogDataFetcher(t)
	if tc.validateAndNormalizeExpected {
		cdf.On("ValidateRequest", mock.Anything, mock.Anything).Return(nil)
		cdf.On("NormalizeRequest", mock.Anything).Return()
	}

	if tc.configureDataFetcher != nil {
		tc.configureDataFetcher(cdf)
	}
	cfg := buildDNSConfig(tc.agentConfig, cdf, tc.mockProcessorError)

	router, err := NewRouter(cfg)
	require.NoError(t, err)

	// Replace the recursor with a mock and configure
	router.recursor = newMockDnsRecursor(t)
	if tc.configureRecursor != nil {
		tc.configureRecursor(router.recursor)
	}

	ctx := tc.requestContext
	if ctx == nil {
		ctx = &Context{}
	}

	var remoteAddress net.Addr
	if tc.remoteAddress != nil {
		remoteAddress = tc.remoteAddress
	} else {
		remoteAddress = &net.UDPAddr{}
	}

	actual := router.HandleRequest(tc.request, *ctx, remoteAddress)
	require.Equal(t, tc.response, actual)
}

func TestRouterDynamicConfig_GetTTLForService(t *testing.T) {
	type testCase struct {
		name             string
		inputKey         string
		shouldMatch      bool
		expectedDuration time.Duration
	}

	testCases := []testCase{
		{
			name:             "strict match",
			inputKey:         "foo",
			shouldMatch:      true,
			expectedDuration: 1 * time.Second,
		},
		{
			name:             "wildcard match",
			inputKey:         "bar",
			shouldMatch:      true,
			expectedDuration: 2 * time.Second,
		},
		{
			name:             "wildcard match 2",
			inputKey:         "bart",
			shouldMatch:      true,
			expectedDuration: 2 * time.Second,
		},
		{
			name:             "no match",
			inputKey:         "homer",
			shouldMatch:      false,
			expectedDuration: 0 * time.Second,
		},
	}

	rtCfg := &config.RuntimeConfig{
		DNSServiceTTL: map[string]time.Duration{
			"foo":  1 * time.Second,
			"bar*": 2 * time.Second,
		},
	}
	cfg, err := getDynamicRouterConfig(rtCfg)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, ok := cfg.GetTTLForService(tc.inputKey)
			require.Equal(t, tc.shouldMatch, ok)
			require.Equal(t, tc.expectedDuration, actual)
		})
	}
}
func buildDNSConfig(agentConfig *config.RuntimeConfig, cdf discovery.CatalogDataFetcher, _ error) Config {
	cfg := Config{
		AgentConfig: &config.RuntimeConfig{
			DNSDomain:  "consul",
			DNSNodeTTL: 123 * time.Second,
			DNSSOA: config.RuntimeSOAConfig{
				Refresh: 1,
				Retry:   2,
				Expire:  3,
				Minttl:  4,
			},
			DNSUDPAnswerLimit: maxUDPAnswerLimit,
		},
		EntMeta:   acl.EnterpriseMeta{},
		Logger:    hclog.NewNullLogger(),
		Processor: discovery.NewQueryProcessor(cdf),
		TokenFunc: func() string { return "" },
		TranslateServiceAddressFunc: func(dc string, address string, taggedAddresses map[string]structs.ServiceAddress, accept dnsutil.TranslateAddressAccept) string {
			return address
		},
		TranslateAddressFunc: func(dc string, addr string, taggedAddresses map[string]string, accept dnsutil.TranslateAddressAccept) string {
			return addr
		},
	}

	if agentConfig != nil {
		cfg.AgentConfig = agentConfig
	}

	return cfg
}

// TestDNS_BinaryTruncate tests the dnsBinaryTruncate function.
func TestDNS_BinaryTruncate(t *testing.T) {
	msgSrc := new(dns.Msg)
	msgSrc.Compress = true
	msgSrc.SetQuestion("redis.service.consul.", dns.TypeSRV)

	for i := 0; i < 5000; i++ {
		target := fmt.Sprintf("host-redis-%d-%d.test.acme.com.node.dc1.consul.", i/256, i%256)
		msgSrc.Answer = append(msgSrc.Answer, &dns.SRV{Hdr: dns.RR_Header{Name: "redis.service.consul.", Class: 1, Rrtype: dns.TypeSRV, Ttl: 0x3c}, Port: 0x4c57, Target: target})
		msgSrc.Extra = append(msgSrc.Extra, &dns.CNAME{Hdr: dns.RR_Header{Name: target, Class: 1, Rrtype: dns.TypeCNAME, Ttl: 0x3c}, Target: fmt.Sprintf("fx.168.%d.%d.", i/256, i%256)})
	}
	for _, compress := range []bool{true, false} {
		for idx, maxSize := range []int{12, 256, 512, 8192, 65535} {
			t.Run(fmt.Sprintf("binarySearch %d", maxSize), func(t *testing.T) {
				msg := new(dns.Msg)
				msgSrc.Compress = compress
				msgSrc.SetQuestion("redis.service.consul.", dns.TypeSRV)
				msg.Answer = msgSrc.Answer
				msg.Extra = msgSrc.Extra
				msg.Ns = msgSrc.Ns
				index := make(map[string]dns.RR, len(msg.Extra))
				indexRRs(msg.Extra, index)
				blen := dnsBinaryTruncate(msg, maxSize, index, true)
				msg.Answer = msg.Answer[:blen]
				syncExtra(index, msg)
				predicted := msg.Len()
				buf, err := msg.Pack()
				if err != nil {
					t.Error(err)
				}
				if predicted < len(buf) {
					t.Fatalf("Bug in DNS library: %d != %d", predicted, len(buf))
				}
				if len(buf) > maxSize || (idx != 0 && len(buf) < 16) {
					t.Fatalf("bad[%d]: %d > %d", idx, len(buf), maxSize)
				}
			})
		}
	}
}

// TestDNS_syncExtra tests the syncExtra function.
func TestDNS_syncExtra(t *testing.T) {
	resp := &dns.Msg{
		Answer: []dns.RR{
			// These two are on the same host so the redundant extra
			// records should get deduplicated.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1002,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			// This one isn't in the Consul domain so it will get a
			// CNAME and then an A record from the recursor.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1003,
				Target: "demo.consul.io.",
			},
			// This one isn't in the Consul domain and it will get
			// a CNAME and A record from a recursor that alters the
			// case of the name. This proves we look up in the index
			// in a case-insensitive way.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "insensitive.consul.io.",
			},
			// This is also a CNAME, but it'll be set up to loop to
			// make sure we don't crash.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "deadly.consul.io.",
			},
			// This is also a CNAME, but it won't have another record.
			&dns.SRV{
				Hdr: dns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: dns.TypeSRV,
					Class:  dns.ClassINET,
				},
				Port:   1001,
				Target: "nope.consul.io.",
			},
		},
		Extra: []dns.RR{
			// These should get deduplicated.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			// This is a normal CNAME followed by an A record but we
			// have flipped the order. The algorithm should emit them
			// in the opposite order.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			// These differ in case to test case insensitivity.
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			// This doesn't appear in the answer, so should get
			// dropped.
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-186.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.186"),
			},
			// These two test edge cases with CNAME handling.
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}

	index := make(map[string]dns.RR)
	indexRRs(resp.Extra, index)
	syncExtra(index, resp)

	expected := &dns.Msg{
		Answer: resp.Answer,
		Extra: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

// getUint32Ptr return the pointer of an uint32 literal
func getUint32Ptr(i uint32) *uint32 {
	return &i
}

func TestRouter_ReloadConfig(t *testing.T) {
	cdf := discovery.NewMockCatalogDataFetcher(t)
	cfg := buildDNSConfig(nil, cdf, nil)
	router, err := NewRouter(cfg)
	require.NoError(t, err)

	router.recursor = newMockDnsRecursor(t)

	// Reload the config
	newAgentConfig := &config.RuntimeConfig{
		DNSARecordLimit:       123,
		DNSEnableTruncate:     true,
		DNSNodeTTL:            234,
		DNSRecursorStrategy:   "strategy-123",
		DNSRecursorTimeout:    345,
		DNSUDPAnswerLimit:     456,
		DNSNodeMetaTXT:        true,
		DNSDisableCompression: true,
		DNSSOA: config.RuntimeSOAConfig{
			Expire:  123,
			Minttl:  234,
			Refresh: 345,
			Retry:   456,
		},
		DNSServiceTTL: map[string]time.Duration{
			"wildcard-config-*": 123,
			"strict-config":     234,
		},
		DNSRecursors: []string{
			"8.8.8.8",
			"2001:4860:4860::8888",
		},
	}

	expectTTLRadix := radix.New()
	expectTTLRadix.Insert("wildcard-config-", time.Duration(123))

	expectedCfg := &RouterDynamicConfig{
		ARecordLimit:       123,
		EnableTruncate:     true,
		NodeTTL:            234,
		RecursorStrategy:   "strategy-123",
		RecursorTimeout:    345,
		UDPAnswerLimit:     456,
		NodeMetaTXT:        true,
		DisableCompression: true,
		SOAConfig: SOAConfig{
			Expire:  123,
			Minttl:  234,
			Refresh: 345,
			Retry:   456,
		},
		TTLRadix: expectTTLRadix,
		TTLStrict: map[string]time.Duration{
			"strict-config": 234,
		},
		Recursors: []string{
			"8.8.8.8:53",
			"[2001:4860:4860::8888]:53",
		},
	}
	err = router.ReloadConfig(newAgentConfig)
	require.NoError(t, err)
	savedCfg := router.dynamicConfig.Load().(*RouterDynamicConfig)

	// Ensure the new config is used
	require.Equal(t, expectedCfg, savedCfg)
}

func Test_isPTRSubdomain(t *testing.T) {
	testCases := []struct {
		name     string
		domain   string
		expected bool
	}{
		{
			name:     "empty domain returns false",
			domain:   "",
			expected: false,
		},
		{
			name:     "last label is 'arpa' returns true",
			domain:   "my-addr.arpa.",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isPTRSubdomain(tc.domain)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func Test_isAddrSubdomain(t *testing.T) {
	testCases := []struct {
		name     string
		domain   string
		expected bool
	}{
		{
			name:     "empty domain returns false",
			domain:   "",
			expected: false,
		},
		{
			name:     "'c000020a.addr.dc1.consul.' returns true",
			domain:   "c000020a.addr.dc1.consul.",
			expected: true,
		},
		{
			name:     "'c000020a.addr.consul.' returns true",
			domain:   "c000020a.addr.consul.",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isAddrSubdomain(tc.domain)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func Test_stripAnyFailoverSuffix(t *testing.T) {
	testCases := []struct {
		name                   string
		target                 string
		expectedEnableFailover bool
		expectedResult         string
	}{
		{
			name:                   "my-addr.service.dc1.consul.failover. returns 'my-addr.service.dc1.consul' and true",
			target:                 "my-addr.service.dc1.consul.failover.",
			expectedEnableFailover: true,
			expectedResult:         "my-addr.service.dc1.consul.",
		},
		{
			name:                   "my-addr.service.dc1.consul.no-failover. returns 'my-addr.service.dc1.consul' and false",
			target:                 "my-addr.service.dc1.consul.no-failover.",
			expectedEnableFailover: false,
			expectedResult:         "my-addr.service.dc1.consul.",
		},
		{
			name:                   "my-addr.service.dc1.consul. returns 'my-addr.service.dc1.consul' and false",
			target:                 "my-addr.service.dc1.consul.",
			expectedEnableFailover: false,
			expectedResult:         "my-addr.service.dc1.consul.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, actualEnableFailover := stripAnyFailoverSuffix(tc.target)
			require.Equal(t, tc.expectedEnableFailover, actualEnableFailover)
			require.Equal(t, tc.expectedResult, actual)
		})
	}
}

func Test_trimDomain(t *testing.T) {
	testCases := []struct {
		name           string
		domain         string
		altDomain      string
		questionName   string
		expectedResult string
	}{
		{
			name:           "given domain is 'consul.' and altDomain is 'my.consul.', when calling trimDomain with 'my-service.my.consul.', it returns 'my-service.'",
			questionName:   "my-service.my.consul.",
			domain:         "consul.",
			altDomain:      "my.consul.",
			expectedResult: "my-service.",
		},
		{
			name:           "given domain is 'consul.' and altDomain is 'my.consul.', when calling trimDomain with 'my-service.consul.', it returns 'my-service.'",
			questionName:   "my-service.consul.",
			domain:         "consul.",
			altDomain:      "my.consul.",
			expectedResult: "my-service.",
		},
		{
			name:           "given domain is 'consul.' and altDomain is 'my-consul.', when calling trimDomain with 'my-service.consul.', it returns 'my-service.'",
			questionName:   "my-service.consul.",
			domain:         "consul.",
			altDomain:      "my-consul.",
			expectedResult: "my-service.",
		},
		{
			name:           "given domain is 'consul.' and altDomain is 'my-consul.', when calling trimDomain with 'my-service.my-consul.', it returns 'my-service.'",
			questionName:   "my-service.my-consul.",
			domain:         "consul.",
			altDomain:      "my-consul.",
			expectedResult: "my-service.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := Router{
				domain:    tc.domain,
				altDomain: tc.altDomain,
			}
			actual := router.trimDomain(tc.questionName)
			require.Equal(t, tc.expectedResult, actual)
		})
	}
}
