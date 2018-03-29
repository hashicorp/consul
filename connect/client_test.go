package connect

// import (
// 	"context"
// 	"crypto/x509"
// 	"crypto/x509/pkix"
// 	"encoding/asn1"
// 	"io/ioutil"
// 	"net"
// 	"net/http"
// 	"net/http/httptest"
// 	"net/url"
// 	"strconv"
// 	"testing"

// 	"github.com/hashicorp/consul/api"
// 	"github.com/hashicorp/consul/testutil"
// 	"github.com/stretchr/testify/require"
// )

// func TestNewInsecureDevClientWithLocalCerts(t *testing.T) {

// 	agent, err := api.NewClient(api.DefaultConfig())
// 	require.Nil(t, err)

// 	got, err := NewInsecureDevClientWithLocalCerts(agent,
// 		"testdata/ca1-ca-consul-internal.cert.pem",
// 		"testdata/ca1-svc-web.cert.pem",
// 		"testdata/ca1-svc-web.key.pem",
// 	)
// 	require.Nil(t, err)

// 	// Sanity check correct certs were loaded
// 	serverCfg, err := got.ServerTLSConfig()
// 	require.Nil(t, err)
// 	caSubjects := serverCfg.RootCAs.Subjects()
// 	require.Len(t, caSubjects, 1)
// 	caSubject, err := testNameFromRawDN(caSubjects[0])
// 	require.Nil(t, err)
// 	require.Equal(t, "Consul Internal", caSubject.CommonName)

// 	require.Len(t, serverCfg.Certificates, 1)
// 	cert, err := x509.ParseCertificate(serverCfg.Certificates[0].Certificate[0])
// 	require.Nil(t, err)
// 	require.Equal(t, "web", cert.Subject.CommonName)
// }

// func testNameFromRawDN(raw []byte) (*pkix.Name, error) {
// 	var seq pkix.RDNSequence
// 	if _, err := asn1.Unmarshal(raw, &seq); err != nil {
// 		return nil, err
// 	}

// 	var name pkix.Name
// 	name.FillFromRDNSequence(&seq)
// 	return &name, nil
// }

// func testAgent(t *testing.T) (*testutil.TestServer, *api.Client) {
// 	t.Helper()

// 	// Make client config
// 	conf := api.DefaultConfig()

// 	// Create server
// 	server, err := testutil.NewTestServerConfigT(t, nil)
// 	require.Nil(t, err)

// 	conf.Address = server.HTTPAddr

// 	// Create client
// 	agent, err := api.NewClient(conf)
// 	require.Nil(t, err)

// 	return server, agent
// }

// func testService(t *testing.T, ca, name string, client *api.Client) *httptest.Server {
// 	t.Helper()

// 	// Run a test service to discover
// 	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Write([]byte("svc: " + name))
// 	}))
// 	server.TLS = TestTLSConfig(t, ca, name)
// 	server.StartTLS()

// 	u, err := url.Parse(server.URL)
// 	require.Nil(t, err)

// 	port, err := strconv.Atoi(u.Port())
// 	require.Nil(t, err)

// 	// If client is passed, register the test service instance
// 	if client != nil {
// 		svc := &api.AgentServiceRegistration{
// 			// TODO(banks): we don't really have a good way to represent
// 			// connect-native apps yet so we have to pretend out little server is a
// 			// proxy for now.
// 			Kind:             api.ServiceKindConnectProxy,
// 			ProxyDestination: name,
// 			Name:             name + "-proxy",
// 			Address:          u.Hostname(),
// 			Port:             port,
// 		}
// 		err := client.Agent().ServiceRegister(svc)
// 		require.Nil(t, err)
// 	}

// 	return server
// }

// func TestDialService(t *testing.T) {
// 	consulServer, agent := testAgent(t)
// 	defer consulServer.Stop()

// 	svc := testService(t, "ca1", "web", agent)
// 	defer svc.Close()

// 	c, err := NewInsecureDevClientWithLocalCerts(agent,
// 		"testdata/ca1-ca-consul-internal.cert.pem",
// 		"testdata/ca1-svc-web.cert.pem",
// 		"testdata/ca1-svc-web.key.pem",
// 	)
// 	require.Nil(t, err)

// 	conn, err := c.DialService(context.Background(), "default", "web")
// 	require.Nilf(t, err, "err: %s", err)

// 	// Inject our conn into http.Transport
// 	httpClient := &http.Client{
// 		Transport: &http.Transport{
// 			DialTLS: func(network, addr string) (net.Conn, error) {
// 				return conn, nil
// 			},
// 		},
// 	}

// 	// Don't be fooled the hostname here is ignored since we did the dialling
// 	// ourselves
// 	resp, err := httpClient.Get("https://web.connect.consul/")
// 	require.Nil(t, err)
// 	defer resp.Body.Close()
// 	body, err := ioutil.ReadAll(resp.Body)
// 	require.Nil(t, err)

// 	require.Equal(t, "svc: web", string(body))
// }
