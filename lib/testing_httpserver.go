package lib

import (
	"net/http"

	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/mitchellh/go-testing-interface"
)

// StartTestServer fires up a web server on a random unused port to serve the
// given handler body. The address it is listening on is returned. When the
// test case terminates the server will be stopped via cleanup functions.
//
// We can't directly use httptest.Server here because that only thinks a port
// is free if it's not bound. Consul tests frequently reserve ports via
// `sdk/freeport` so you can have one part of the test try to use a port and
// _know_ nothing is listening. If you simply assumed unbound ports were free
// you'd end up with test cross-talk and weirdness.
func StartTestServer(t testing.T, handler http.Handler) string {
	addr := ipaddr.FormatAddressPort("127.0.0.1", freeport.Port(t))

	server := &http.Server{Addr: addr, Handler: handler}
	t.Cleanup(func() {
		server.Close()
	})

	go server.ListenAndServe()

	return addr
}
