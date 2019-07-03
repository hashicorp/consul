package trace

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestTraceParse(t *testing.T) {
	tests := []struct {
		input        string
		shouldErr    bool
		endpoint     string
		every        uint64
		serviceName  string
		clientServer bool
	}{
		// oks
		{`trace`, false, "http://localhost:9411/api/v1/spans", 1, `coredns`, false},
		{`trace localhost:1234`, false, "http://localhost:1234/api/v1/spans", 1, `coredns`, false},
		{`trace http://localhost:1234/somewhere/else`, false, "http://localhost:1234/somewhere/else", 1, `coredns`, false},
		{`trace zipkin localhost:1234`, false, "http://localhost:1234/api/v1/spans", 1, `coredns`, false},
		{`trace datadog localhost`, false, "localhost", 1, `coredns`, false},
		{`trace datadog http://localhost:8127`, false, "http://localhost:8127", 1, `coredns`, false},
		{"trace {\n every 100\n}", false, "http://localhost:9411/api/v1/spans", 100, `coredns`, false},
		{"trace {\n every 100\n service foobar\nclient_server\n}", false, "http://localhost:9411/api/v1/spans", 100, `foobar`, true},
		{"trace {\n every 2\n client_server true\n}", false, "http://localhost:9411/api/v1/spans", 2, `coredns`, true},
		{"trace {\n client_server false\n}", false, "http://localhost:9411/api/v1/spans", 1, `coredns`, false},
		// fails
		{`trace footype localhost:4321`, true, "", 1, "", false},
		{"trace {\n every 2\n client_server junk\n}", true, "", 1, "", false},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		m, err := traceParse(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}

		if test.shouldErr {
			continue
		}

		if test.endpoint != m.Endpoint {
			t.Errorf("Test %v: Expected endpoint %s but found: %s", i, test.endpoint, m.Endpoint)
		}
		if test.every != m.every {
			t.Errorf("Test %v: Expected every %d but found: %d", i, test.every, m.every)
		}
		if test.serviceName != m.serviceName {
			t.Errorf("Test %v: Expected service name %s but found: %s", i, test.serviceName, m.serviceName)
		}
		if test.clientServer != m.clientServer {
			t.Errorf("Test %v: Expected client_server %t but found: %t", i, test.clientServer, m.clientServer)
		}
	}
}
