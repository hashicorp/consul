package proxy

import (
	"fmt"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	"github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"context"

	"github.com/miekg/dns"
	"google.golang.org/grpc/grpclog"
)

func init() {
	grpclog.SetLoggerV2(discardV2{})
}

func buildPool(size int) ([]*healthcheck.UpstreamHost, func(), error) {
	ups := make([]*healthcheck.UpstreamHost, size)
	srvs := []*dns.Server{}
	errs := []error{}
	for i := 0; i < size; i++ {
		srv, addr, err := test.TCPServer("localhost:0")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ups[i] = &healthcheck.UpstreamHost{Name: addr}
		srvs = append(srvs, srv)
	}
	stopIt := func() {
		for _, s := range srvs {
			s.Shutdown()
		}
	}
	if len(errs) > 0 {
		go stopIt()
		valErr := ""
		for _, e := range errs {
			valErr += fmt.Sprintf("%v\n", e)
		}
		return nil, nil, fmt.Errorf("error at allocation of the pool : %v", valErr)
	}
	return ups, stopIt, nil
}

func TestGRPCStartupShutdown(t *testing.T) {

	pool, closePool, err := buildPool(2)
	if err != nil {
		t.Fatalf("error creating the pool of upstream for the test : %s", err)
	}
	defer closePool()

	upstream := &staticUpstream{
		from: ".",
		HealthCheck: healthcheck.HealthCheck{
			Hosts: pool,
		},
	}
	g := newGrpcClient(nil, upstream)
	upstream.ex = g

	p := &Proxy{}
	p.Upstreams = &[]Upstream{upstream}

	err = g.OnStartup(p)
	if err != nil {
		t.Fatalf("Error starting grpc client exchanger: %s", err)
	}
	if len(g.clients) != len(pool) {
		t.Fatalf("Expected %d grpc clients but found %d", len(pool), len(g.clients))
	}

	err = g.OnShutdown(p)
	if err != nil {
		t.Fatalf("Error stopping grpc client exchanger: %s", err)
	}
	if len(g.clients) != 0 {
		t.Errorf("Shutdown didn't remove clients, found %d", len(g.clients))
	}
	if len(g.conns) != 0 {
		t.Errorf("Shutdown didn't remove conns, found %d", len(g.conns))
	}
}

func TestGRPCRunAQuery(t *testing.T) {

	pool, closePool, err := buildPool(2)
	if err != nil {
		t.Fatalf("error creating the pool of upstream for the test : %s", err)
	}
	defer closePool()

	upstream := &staticUpstream{
		from: ".",
		HealthCheck: healthcheck.HealthCheck{
			Hosts: pool,
		},
	}
	g := newGrpcClient(nil, upstream)
	upstream.ex = g

	p := &Proxy{}
	p.Upstreams = &[]Upstream{upstream}

	err = g.OnStartup(p)
	if err != nil {
		t.Fatalf("Error starting grpc client exchanger: %s", err)
	}
	// verify the client is usable, or an error is properly raised
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	g.Exchange(context.TODO(), "localhost:10053", state)

	// verify that you have proper error if the hostname is unknwn or not registered
	_, err = g.Exchange(context.TODO(), "invalid:10055", state)
	if err == nil {
		t.Errorf("Expecting a proper error when querying gRPC client with invalid hostname : %s", err)
	}

	err = g.OnShutdown(p)
	if err != nil {
		t.Fatalf("Error stopping grpc client exchanger: %s", err)
	}
}

func TestGRPCRunAQueryOnSecureLinkWithInvalidCert(t *testing.T) {

	pool, closePool, err := buildPool(1)
	if err != nil {
		t.Fatalf("error creating the pool of upstream for the test : %s", err)
	}
	defer closePool()

	upstream := &staticUpstream{
		from: ".",
		HealthCheck: healthcheck.HealthCheck{
			Hosts: pool,
		},
	}

	filename, rmFunc, err := test.TempFile("", aCert)
	if err != nil {
		t.Errorf("Error saving file : %s", err)
		return
	}
	defer rmFunc()

	tls, _ := tls.NewTLSClientConfig(filename)
	// ignore error as the certificate is known valid

	g := newGrpcClient(tls, upstream)
	upstream.ex = g

	p := &Proxy{}
	p.Upstreams = &[]Upstream{upstream}

	// Although dial will not work, it is not expected to have an error
	err = g.OnStartup(p)
	if err != nil {
		t.Fatalf("Error starting grpc client exchanger: %s", err)
	}

	// verify that you have proper error if the hostname is unknwn or not registered
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}
	_, err = g.Exchange(context.TODO(), pool[0].Name+"-whatever", state)
	if err == nil {
		t.Errorf("Error in Exchange process : %s ", err)
	}

	err = g.OnShutdown(p)
	if err != nil {
		t.Fatalf("Error stopping grpc client exchanger: %s", err)
	}
}

// discard is a Logger that outputs nothing.
type discardV2 struct{}

func (d discardV2) Info(args ...interface{})                    {}
func (d discardV2) Infoln(args ...interface{})                  {}
func (d discardV2) Infof(format string, args ...interface{})    {}
func (d discardV2) Warning(args ...interface{})                 {}
func (d discardV2) Warningln(args ...interface{})               {}
func (d discardV2) Warningf(format string, args ...interface{}) {}
func (d discardV2) Error(args ...interface{})                   {}
func (d discardV2) Errorln(args ...interface{})                 {}
func (d discardV2) Errorf(format string, args ...interface{})   {}
func (d discardV2) Fatal(args ...interface{})                   {}
func (d discardV2) Fatalln(args ...interface{})                 {}
func (d discardV2) Fatalf(format string, args ...interface{})   {}
func (d discardV2) V(l int) bool                                { return true }

const (
	aCert = `-----BEGIN CERTIFICATE-----
	MIIDlDCCAnygAwIBAgIJAPaRnBJUE/FVMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTcxMTI0MTM0OTQ3WhcNMTgxMTI0MTM0OTQ3WjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAuTDeAoWS6tdZVcp/Vh3FlagbC+9Ohi5VjRXgkpcn9JopbcF5s2jpl1v+
cRpqkrmNNKLh8qOhmgdZQdh185VNe/iZ94H42qwKZ48vvnC5hLkk3MdgUT2ewgup
vZhy/Bb1bX+buCWkQa1u8SIilECMIPZHhBP4TuBUKJWK8bBEFAeUnxB5SCkX+un4
pctRlcfg8sX/ghADnp4e//YYDqex+1wQdFqM5zWhWDZAzc5Kdkyy9r+xXNfo4s1h
fI08f6F4skz1koxG2RXOzQ7OK4YxFwT2J6V72iyzUIlRGZTbYDvair/zm1kjTF1R
B1B+XLJF9oIB4BMZbekf033ZVaQ8YwIDAQABo4GGMIGDMDMGA1UdEQQsMCqHBH8A
AAGHBDR3AQGHBDR3AQCHBDR3KmSHBDR3KGSHBDR3KmWHBDR3KtIwHQYDVR0OBBYE
FFAEccLm7D/rN3fEe1fwzH7p0spAMB8GA1UdIwQYMBaAFFAEccLm7D/rN3fEe1fw
zH7p0spAMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBAF4zqaucNcK2
GwYfijwbbtgMqPEvbReUEXsC65riAPjksJQ9L2YxQ7K0RIugRizuD1DNQam+FSb0
cZEMEKzvMUIexbhZNFINWXY2X9yUS/oZd5pWP0WYIhn6qhmLvzl9XpxNPVzBXYWe
duMECCigU2x5tAGmFa6g/pXXOoZCBRzFXwXiuNhSyhJEEwODjLZ6vgbySuU2jso3
va4FKFDdVM16s1/RYOK5oM48XytCMB/JoYoSJHPfpt8LpVNAQEHMvPvHwuZBON/z
q8HFtDjT4pBpB8AfuzwtUZ/zJ5atwxa5+ahcqRnK2kX2RSINfyEy43FZjLlvjcGa
UIRTUJK1JKg=
-----END CERTIFICATE-----`
)
