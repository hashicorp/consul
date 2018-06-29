package variables

import (
	"bytes"
	"testing"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestGetValue(t *testing.T) {
	// test.ResponseWriter has the following values:
	// 		The remote will always be 10.240.0.1 and port 40212.
	// 		The local address is always 127.0.0.1 and port 53.
	tests := []struct {
		varName       string
		expectedValue []byte
		shouldErr     bool
	}{
		{
			queryName,
			[]byte("example.com."),
			false,
		},
		{
			queryType,
			[]byte{0x00, 0x01},
			false,
		},
		{
			clientIP,
			[]byte{10, 240, 0, 1},
			false,
		},
		{
			clientPort,
			[]byte{0x9D, 0x14},
			false,
		},
		{
			protocol,
			[]byte("udp"),
			false,
		},
		{
			serverIP,
			[]byte{127, 0, 0, 1},
			false,
		},
		{
			serverPort,
			[]byte{0, 53},
			false,
		},
		{
			"wrong_var",
			[]byte{},
			true,
		},
	}

	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion("example.com.", dns.TypeA)
		m.Question[0].Qclass = dns.ClassINET
		state := request.Request{W: &test.ResponseWriter{}, Req: m}

		value, err := GetValue(state, tc.varName)

		if tc.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error, but didn't recieve", i)
		}
		if !tc.shouldErr && err != nil {
			t.Errorf("Test %d: Expected no error, but got error: %v", i, err.Error())
		}

		if !bytes.Equal(tc.expectedValue, value) {
			t.Errorf("Test %d: Expected %v but got %v", i, tc.expectedValue, value)
		}
	}
}
