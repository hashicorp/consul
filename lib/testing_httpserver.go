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
func StartTestServer(t testing.T, handler http.Handler) string {
	ports := freeport.MustTake(1)
	t.Cleanup(func() {
		freeport.Return(ports)
	})

	addr := ipaddr.FormatAddressPort("127.0.0.1", ports[0])

	server := &http.Server{Addr: addr, Handler: handler}
	t.Cleanup(func() {
		server.Close()
	})

	go server.ListenAndServe()

	return addr
}
