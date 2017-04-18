package dnsserver

import (
	"testing"
)

func makeConfig(transport string) *Config {
	return &Config{Zone: "example.com", Transport: transport, ListenHost: "127.0.0.1", Port: "53"}
}

func TestNewServer(t *testing.T) {
	_, err := NewServer("127.0.0.1:53", []*Config{makeConfig("dns")})
	if err != nil {
		t.Errorf("Expected no error for NewServer, got %s.", err)
	}

	_, err = NewServergRPC("127.0.0.1:53", []*Config{makeConfig("grpc")})
	if err != nil {
		t.Errorf("Expected no error for NewServergRPC, got %s.", err)
	}

	_, err = NewServerTLS("127.0.0.1:53", []*Config{makeConfig("tls")})
	if err != nil {
		t.Errorf("Expected no error for NewServerTLS, got %s.", err)
	}
}
