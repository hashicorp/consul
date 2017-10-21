package dnsserver

import (
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type testPlugin struct{}

func (tp testPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return 0, nil
}

func (tp testPlugin) Name() string { return "testplugin" }

func testConfig(transport string) *Config {
	c := &Config{
		Zone:       "example.com.",
		Transport:  transport,
		ListenHost: "127.0.0.1",
		Port:       "53",
		Debug:      false,
	}

	c.AddPlugin(func(next plugin.Handler) plugin.Handler { return testPlugin{} })
	return c
}

func TestNewServer(t *testing.T) {
	_, err := NewServer("127.0.0.1:53", []*Config{testConfig("dns")})
	if err != nil {
		t.Errorf("Expected no error for NewServer, got %s.", err)
	}

	_, err = NewServergRPC("127.0.0.1:53", []*Config{testConfig("grpc")})
	if err != nil {
		t.Errorf("Expected no error for NewServergRPC, got %s.", err)
	}

	_, err = NewServerTLS("127.0.0.1:53", []*Config{testConfig("tls")})
	if err != nil {
		t.Errorf("Expected no error for NewServerTLS, got %s.", err)
	}
}

func BenchmarkCoreServeDNS(b *testing.B) {
	s, err := NewServer("127.0.0.1:53", []*Config{testConfig("dns")})
	if err != nil {
		b.Errorf("Expected no error for NewServer, got %s.", err)
	}

	ctx := context.TODO()
	w := &test.ResponseWriter{}
	m := new(dns.Msg)
	m.SetQuestion("aaa.example.com.", dns.TypeTXT)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ServeDNS(ctx, w, m)
	}
}
