// +build k8s

package test

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

var dnsTestCasesFallthrough = []test.Case{
	{
		Qname: "ext-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("example.net.		303	IN	A	13.14.15.16"),
			test.CNAME("ext-svc.test-1.svc.cluster.local. 303 IN	CNAME	example.net."),
		},
	},
	{
		Qname: "f.b.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("f.b.svc.cluster.local.      303    IN      A       10.10.10.11"),
		},
		Ns: []dns.RR{
			test.NS("cluster.local.	303	IN	NS	a.iana-servers.net."),
			test.NS("cluster.local.	303	IN	NS	b.iana-servers.net."),
		},
	},
	{
		Qname: "foo.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("foo.cluster.local.      303    IN      A       10.10.10.10"),
		},
		Ns: []dns.RR{
			test.NS("cluster.local.	303	IN	NS	a.iana-servers.net."),
			test.NS("cluster.local.	303	IN	NS	b.iana-servers.net."),
		},
	},
}

func TestKubernetesFallthrough(t *testing.T) {
	dbfile, rmFunc, err := TempFile(os.TempDir(), clusterLocal)
	if err != nil {
		t.Fatalf("Could not create zonefile for fallthrough server: %s", err)
	}
	defer rmFunc()

	rmFunc, upstream, udp := upstreamServer(t)
	defer upstream.Stop()
	defer rmFunc()

	corefile :=
		`.:0 {
    file ` + dbfile + ` cluster.local
    kubernetes cluster.local {
                endpoint http://localhost:8080
		namespaces test-1
		upstream ` + udp + `
		fallthrough
    }
`
	doIntegrationTests(t, corefile, dnsTestCasesFallthrough)
}
