package lib

import (
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
)

// NewHTTPTestServer starts and returns an httptest.Server that is listening
// on a random port from freeport.Port. When the test case ends the server
// will be stopped.
//
// We can't directly use httptest.Server here because that only thinks a port
// is free if it's not bound. Consul tests frequently reserve ports via
// `sdk/freeport` so you can have one part of the test try to use a port and
// _know_ nothing is listening. If you simply assumed unbound ports were free
// you'd end up with test cross-talk and weirdness.
func NewHTTPTestServer(t freeport.TestingT, handler http.Handler) *httptest.Server {
	srv := httptest.NewUnstartedServer(handler)

	addr := ipaddr.FormatAddressPort("127.0.0.1", freeport.Port(t))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen on %v", addr)
	}
	srv.Listener = listener
	t.Cleanup(srv.Close)
	srv.Start()
	return srv
}
