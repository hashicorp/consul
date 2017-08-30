// +build k8s

package test

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/middleware/test"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

var dnsTestCases = []test.Case{
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      5    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "bogusservice.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "bogusendpoint.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "bogusendpoint.headless-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "svc-1-a.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.*.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "svc-1-a.any.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.any.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "bogusservice.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "bogusservice.any.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "*.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: append([]dns.RR{
			test.A("*.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("*.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("*.test-1.svc.cluster.local.      303    IN      A       10.0.0.115"),
			test.CNAME("*.test-1.svc.cluster.local.  303    IN	     CNAME	  example.net."),
			test.A("example.net.		303	IN	A	13.14.15.16"),
		}, headlessAResponse("*.test-1.svc.cluster.local.", "headless-svc", "test-1")...),
	},
	{
		Qname: "any.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: append([]dns.RR{
			test.A("any.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("any.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("any.test-1.svc.cluster.local.      303    IN      A       10.0.0.115"),
			test.CNAME("any.test-1.svc.cluster.local.  303    IN	     CNAME	  example.net."),
			test.A("example.net.		303	IN	A	13.14.15.16"),
		}, headlessAResponse("any.test-1.svc.cluster.local.", "headless-svc", "test-1")...),
	},
	{
		Qname: "any.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "*.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "*.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: append([]dns.RR{
			test.A("*.*.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("*.*.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("*.*.svc.cluster.local.      303    IN      A       10.0.0.115"),
			test.CNAME("*.*.svc.cluster.local.  303    IN	     CNAME	  example.net."),
			test.A("example.net.		303	IN	A	13.14.15.16"),
		}, headlessAResponse("*.*.svc.cluster.local.", "headless-svc", "test-1")...),
	},
	{
		Qname: "headless-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeSuccess,
		Answer: headlessAResponse("headless-svc.test-1.svc.cluster.local.", "headless-svc", "test-1"),
	},
	{
		Qname: "*._TcP.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("*._TcP.svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	 0  50  443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*._TcP.svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	 0  50   80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303 	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "*.*.bogusservice.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "*.any.svc-1-a.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("*.any.svc-1-a.*.svc.cluster.local.      303    IN    SRV 0 50 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*.any.svc-1-a.*.svc.cluster.local.      303    IN    SRV 0 50 80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "ANY.*.svc-1-a.any.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("ANY.*.svc-1-a.any.svc.cluster.local.      303    IN    SRV 0 50 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("ANY.*.svc-1-a.any.svc.cluster.local.      303    IN    SRV 0 50 80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303 	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "*.*.bogusservice.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "*.*.bogusservice.any.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "_c-port._UDP.*.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: append(srvResponse("_c-port._UDP.*.test-1.svc.cluster.local.", dns.TypeSRV, "headless-svc", "test-1"),
			[]dns.RR{
				test.SRV("_c-port._UDP.*.test-1.svc.cluster.local.      303    IN    SRV 0 33 1234 svc-c.test-1.svc.cluster.local.")}...),
		Extra: append(srvResponse("_c-port._UDP.*.test-1.svc.cluster.local.", dns.TypeA, "headless-svc", "test-1"),
			[]dns.RR{
				test.A("svc-c.test-1.svc.cluster.local.	303	IN	A	10.0.0.115")}...),
	},
	{
		Qname: "*._tcp.any.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("*._tcp.any.test-1.svc.cluster.local.      303    IN    SRV 0 33 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*._tcp.any.test-1.svc.cluster.local.      303    IN    SRV 0 33 80  svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*._tcp.any.test-1.svc.cluster.local.      303    IN    SRV 0 33 80  svc-1-b.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.	303	IN	A	10.0.0.110"),
		},
	},
	{
		Qname: "*.*.any.test-2.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "*.*.*.test-2.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "_http._tcp.*.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.*.*.svc.cluster.local.      303    IN    SRV 0 50 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_http._tcp.*.*.svc.cluster.local.      303    IN    SRV 0 50 80 svc-1-b.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.	303	IN	A	10.0.0.110"),
		},
	},
	{
		Qname: "*.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeSuccess,
		Answer: srvResponse("*.svc-1-a.test-1.svc.cluster.local.", dns.TypeSRV, "svc-1-a", "test-1"),
		Extra:  srvResponse("*.svc-1-a.test-1.svc.cluster.local.", dns.TypeA, "svc-1-a", "test-1"),
	},
	{
		Qname: "*._not-udp-or-tcp.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 1502313310 7200 1800 86400 60"),
		},
	},
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	0 50 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	0 50 80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeServerFailure,
	},
	{
		Qname: "dns-version.cluster.local.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.TXT(`dns-version.cluster.local. 303 IN TXT "1.0.1"`),
		},
	},
	{
		Qname: "ext-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("example.net.		303	IN	A	13.14.15.16"),
			test.CNAME("ext-svc.test-1.svc.cluster.local. 303 IN	CNAME	example.net."),
		},
	},
	{
		Qname: "ext-svc.test-1.svc.cluster.local.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.CNAME("ext-svc.test-1.svc.cluster.local. 303 IN	CNAME	example.net."),
		},
	},
}

var dnsTestCasesFallthrough = []test.Case{
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "bogusservice.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	 303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "bogusendpoint.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "bogusendpoint.headless-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "svc-1-a.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.*.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "svc-1-a.any.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc-1-a.any.svc.cluster.local.      303    IN      A       10.0.0.100"),
		},
	},
	{
		Qname: "bogusservice.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "bogusservice.any.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "*.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: append([]dns.RR{
			test.A("*.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("*.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("*.test-1.svc.cluster.local.      303    IN      A       10.0.0.115"),
			test.CNAME("*.test-1.svc.cluster.local.  303    IN	     CNAME	  example.net."),
			test.A("example.net.		303	IN	A	13.14.15.16"),
		}, headlessAResponse("*.test-1.svc.cluster.local.", "headless-svc", "test-1")...),
	},
	{
		Qname: "any.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: append([]dns.RR{
			test.A("any.test-1.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("any.test-1.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("any.test-1.svc.cluster.local.      303    IN      A       10.0.0.115"),
			test.CNAME("any.test-1.svc.cluster.local.  303    IN	     CNAME	  example.net."),
			test.A("example.net.		303	IN	A	13.14.15.16"),
		}, headlessAResponse("any.test-1.svc.cluster.local.", "headless-svc", "test-1")...),
	},
	{
		Qname: "any.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "*.test-2.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "*.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: append([]dns.RR{
			test.A("*.*.svc.cluster.local.      303    IN      A       10.0.0.100"),
			test.A("*.*.svc.cluster.local.      303    IN      A       10.0.0.110"),
			test.A("*.*.svc.cluster.local.      303    IN      A       10.0.0.115"),
			test.CNAME("*.*.svc.cluster.local.  303    IN	     CNAME	  example.net."),
			test.A("example.net.		303	IN	A	13.14.15.16"),
		}, headlessAResponse("*.*.svc.cluster.local.", "headless-svc", "test-1")...),
	},
	{
		Qname: "headless-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeSuccess,
		Answer: headlessAResponse("headless-svc.test-1.svc.cluster.local.", "headless-svc", "test-1"),
	},
	{
		Qname: "*._TcP.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("*._TcP.svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	 0  50  443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*._TcP.svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	 0  50   80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "*.*.bogusservice.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "*.any.svc-1-a.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("*.any.svc-1-a.*.svc.cluster.local.      303    IN    SRV 0 50 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*.any.svc-1-a.*.svc.cluster.local.      303    IN    SRV 0 50  80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "ANY.*.svc-1-a.any.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("ANY.*.svc-1-a.any.svc.cluster.local.      303    IN    SRV 0 50 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("ANY.*.svc-1-a.any.svc.cluster.local.      303    IN    SRV 0 50  80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "*.*.bogusservice.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "*.*.bogusservice.any.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "_c-port._UDP.*.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: append(srvResponse("_c-port._UDP.*.test-1.svc.cluster.local.", dns.TypeSRV, "headless-svc", "test-1"),
			[]dns.RR{
				test.SRV("_c-port._UDP.*.test-1.svc.cluster.local.      303    IN    SRV 0 33 1234 svc-c.test-1.svc.cluster.local.")}...),
		Extra: append(srvResponse("_c-port._UDP.*.test-1.svc.cluster.local.", dns.TypeA, "headless-svc", "test-1"),
			[]dns.RR{
				test.A("svc-c.test-1.svc.cluster.local.	303	IN	A	10.0.0.115"),
			}...),
	},
	{
		Qname: "*._tcp.any.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("*._tcp.any.test-1.svc.cluster.local.      303    IN    SRV 0 33 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*._tcp.any.test-1.svc.cluster.local.      303    IN    SRV 0 33  80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("*._tcp.any.test-1.svc.cluster.local.      303    IN    SRV 0 33  80 svc-1-b.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.	303	IN	A	10.0.0.110"),
		},
	},
	{
		Qname: "*.*.any.test-2.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "*.*.*.test-2.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "_http._tcp.*.*.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.*.*.svc.cluster.local.      303    IN    SRV 0 50 80 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("_http._tcp.*.*.svc.cluster.local.      303    IN    SRV 0 50 80 svc-1-b.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
			test.A("svc-1-b.test-1.svc.cluster.local.	303	IN	A	10.0.0.110"),
		},
	},
	{
		Qname: "*.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode:  dns.RcodeSuccess,
		Answer: srvResponse("*.svc-1-a.test-1.svc.cluster.local.", dns.TypeSRV, "svc-1-a", "test-1"),
		Extra:  srvResponse("*.svc-1-a.test-1.svc.cluster.local.", dns.TypeA, "svc-1-a", "test-1"),
	},
	{
		Qname: "*._not-udp-or-tcp.svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("cluster.local.	303	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600"),
		},
	},
	{
		Qname: "svc-1-a.test-1.svc.cluster.local.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	0 50 443 svc-1-a.test-1.svc.cluster.local."),
			test.SRV("svc-1-a.test-1.svc.cluster.local.	303	IN	SRV	0 50 80 svc-1-a.test-1.svc.cluster.local."),
		},
		Extra: []dns.RR{
			test.A("svc-1-a.test-1.svc.cluster.local.	303	IN	A	10.0.0.100"),
		},
	},
	{
		Qname: "10-20-0-101.test-1.pod.cluster.local.", Qtype: dns.TypeA,
		Rcode:  dns.RcodeServerFailure,
		Answer: []dns.RR{},
	},
	{
		Qname: "dns-version.cluster.local.", Qtype: dns.TypeTXT,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.TXT("dns-version.cluster.local. 28800 IN TXT \"1.0.1\""),
		},
	},
	{
		Qname: "ext-svc.test-1.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("example.net.		303	IN	A	13.14.15.16"),
			test.CNAME("ext-svc.test-1.svc.cluster.local. 303 IN	CNAME	example.net."),
		},
	},
	{
		Qname: "ext-svc.test-1.svc.cluster.local.", Qtype: dns.TypeCNAME,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
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

func doIntegrationTests(t *testing.T, corefile string, testCases []test.Case) {
	server, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer server.Stop()

	// Work-around for timing condition that results in no-data being returned in test environment.
	time.Sleep(3 * time.Second)

	for _, tc := range testCases {

		c := new(dns.Client)
		m := tc.Msg()

		res, _, err := c.Exchange(m, udp)
		if err != nil {
			t.Fatalf("Could not send query: %s", err)
		}

		// Before sorting, make sure that CNAMES do not appear after their target records and then sort the tc.
		test.CNAMEOrder(t, res)
		sort.Sort(test.RRSet(tc.Answer))
		sort.Sort(test.RRSet(tc.Ns))
		sort.Sort(test.RRSet(tc.Extra))

		test.SortAndCheck(t, res, tc)
	}
}

func createUpstreamServer(t *testing.T) (func(), *caddy.Instance, string) {
	upfile, rmfile, err := TempFile(os.TempDir(), exampleNet)
	if err != nil {
		t.Fatalf("Could not create file for CNAME upstream lookups: %s", err)
	}
	upstreamServerCorefile := `.:0 {
    file ` + upfile + ` example.net
	erratic . {
		drop 0
	}
	`
	server, udp, _, err := CoreDNSServerAndPorts(upstreamServerCorefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	return rmfile, server, udp
}

func TestKubernetesIntegration(t *testing.T) {

	removeUpstreamConfig, upstreamServer, udp := createUpstreamServer(t)
	defer upstreamServer.Stop()
	defer removeUpstreamConfig()

	corefile :=
		`.:0 {
    kubernetes cluster.local 0.0.10.in-addr.arpa {
                endpoint http://localhost:8080
		namespaces test-1
		pods disabled
		upstream ` + udp + `
    }
	erratic . {
		drop 0
	}
`
	doIntegrationTests(t, corefile, dnsTestCases)
}

func TestKubernetesIntegrationFallthrough(t *testing.T) {
	dbfile, rmFunc, err := TempFile(os.TempDir(), clusterLocal)
	if err != nil {
		t.Fatalf("Could not create TempFile for fallthrough: %s", err)
	}
	defer rmFunc()

	removeUpstreamConfig, upstreamServer, udp := createUpstreamServer(t)
	defer upstreamServer.Stop()
	defer removeUpstreamConfig()

	corefile :=
		`.:0 {
    file ` + dbfile + ` cluster.local
    kubernetes cluster.local {
                endpoint http://localhost:8080
		namespaces test-1
		upstream ` + udp + `
		fallthrough
    }
    erratic {
	drop 0
    }
`
	doIntegrationTests(t, corefile, dnsTestCasesFallthrough)
}

// headlessAResponse returns the answer to an A request for the specific name and namespace.
func headlessAResponse(qname, namespace, name string) []dns.RR {
	rr := []dns.RR{}

	str, err := endpointIPs(name, namespace)
	if err != nil {
		log.Fatal("Error running kubectl command: ", err.Error())
	}
	result := strings.Split(string(str), " ")
	lr := len(result)

	for i := 0; i < lr; i++ {
		rr = append(rr, test.A(qname+"    303    IN      A   "+result[i]))
	}
	return rr
}

// srvResponse returns the answer to a SRV request for the specific name and namespace
// qtype is the type of answer to generate, eg: TypeSRV (for answer section) or TypeA (for extra section).
func srvResponse(qname string, qtype uint16, namespace, name string) []dns.RR {
	rr := []dns.RR{}

	str, err := endpointIPs(name, namespace)

	if err != nil {
		log.Fatal("Error running kubectl command: ", err.Error())
	}
	result := strings.Split(string(str), " ")
	lr := len(result)

	for i := 0; i < lr; i++ {
		ip := strings.Replace(result[i], ".", "-", -1)
		t := strconv.Itoa(100 / (lr + 1))

		switch qtype {
		case dns.TypeA:
			rr = append(rr, test.A(ip+"."+namespace+"."+name+".svc.cluster.local.	303	IN	A	"+result[i]))
		case dns.TypeSRV:
			if namespace == "headless-svc" {
				rr = append(rr, test.SRV(qname+"   303    IN    SRV 0 "+t+" 1234  "+ip+"."+namespace+"."+name+".svc.cluster.local."))
			} else {
				rr = append(rr, test.SRV(qname+"   303    IN    SRV 0 "+t+" 443  "+ip+"."+namespace+"."+name+".svc.cluster.local."))
				rr = append(rr, test.SRV(qname+"   303    IN    SRV 0 "+t+" 80  "+ip+"."+namespace+"."+name+".svc.cluster.local."))
			}
		}
	}
	return rr
}

//endpointIPs retrieves the IP address for a given name and namespace by parsing json using kubectl command
func endpointIPs(name, namespace string) (cmdOut []byte, err error) {

	kctl := os.Getenv("KUBECTL")

	if kctl == "" {
		kctl = "kubectl"
	}
	cmdArgs := kctl + " -n " + name + " get endpoints " + namespace + " -o jsonpath={.subsets[*].addresses[*].ip}"
	if cmdOut, err = exec.Command("sh", "-c", cmdArgs).Output(); err != nil {
		return nil, err
	}
	return cmdOut, nil
}

const clusterLocal = `; cluster.local test file for fallthrough
cluster.local.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
cluster.local.		IN	NS	b.iana-servers.net.
cluster.local.		IN	NS	a.iana-servers.net.
cluster.local.		IN	A	127.0.0.1
cluster.local.		IN	A	127.0.0.2
foo.cluster.local.      IN      A	10.10.10.10
f.b.svc.cluster.local.  IN      A	10.10.10.11
*.w.cluster.local.      IN      TXT     "Wildcard"
a.b.svc.cluster.local.  IN      TXT     "Not a wildcard"
cname.cluster.local.    IN      CNAME   www.example.net.

service.namespace.svc.cluster.local.    IN      SRV     8080 10 10 cluster.local.
`

const exampleNet = `; example.net. test file for cname tests
example.net.          IN      SOA     ns.example.net. admin.example.net. 2015082541 7200 3600 1209600 3600
example.net. IN A 13.14.15.16
`
