package lib

import (
	"net/http"

	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/sdk/freeport"
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
func StartTestServer(handler http.Handler) (string, func()) {
	ports := freeport.MustTake(1)
	addr := ipaddr.FormatAddressPort("127.0.0.1", ports[0])

	server := &http.Server{Addr: addr, Handler: handler}
	go server.ListenAndServe()

	return addr, func() {
		server.Close()
		freeport.Return(ports)
	}
}
