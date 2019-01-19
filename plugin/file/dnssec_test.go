package file

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// All OPT RR are added in server.go, so we don't specify them in the unit tests.
var dnssecTestCases = []test.Case{
	{
		Qname: "miek.nl.", Qtype: dns.TypeSOA, Do: true,
		Answer: []dns.RR{
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
		Ns: auth,
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeAAAA, Do: true,
		Answer: []dns.RR{
			test.AAAA("miek.nl.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fef1:6735"),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	AAAA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. SsRT="),
		},
		Ns: auth,
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeNS, Do: true,
		Answer: []dns.RR{
			test.NS("miek.nl.	1800	IN	NS	ext.ns.whyscream.net."),
			test.NS("miek.nl.	1800	IN	NS	linode.atoom.net."),
			test.NS("miek.nl.	1800	IN	NS	ns-ext.nlnetlabs.nl."),
			test.NS("miek.nl.	1800	IN	NS	omval.tednet.nl."),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	NS 8 2 1800 20160426031301 20160327031301 12051 miek.nl. ZLtsQhwaz+lHfNpztFoR1Vxs="),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeMX, Do: true,
		Answer: []dns.RR{
			test.MX("miek.nl.	1800	IN	MX	1 aspmx.l.google.com."),
			test.MX("miek.nl.	1800	IN	MX	10 aspmx2.googlemail.com."),
			test.MX("miek.nl.	1800	IN	MX	10 aspmx3.googlemail.com."),
			test.MX("miek.nl.	1800	IN	MX	5 alt1.aspmx.l.google.com."),
			test.MX("miek.nl.	1800	IN	MX	5 alt2.aspmx.l.google.com."),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	MX 8 2 1800 20160426031301 20160327031301 12051 miek.nl. kLqG+iOr="),
		},
		Ns: auth,
	},
	{
		Qname: "www.miek.nl.", Qtype: dns.TypeA, Do: true,
		Answer: []dns.RR{
			test.A("a.miek.nl.	1800	IN	A	139.162.196.78"),
			test.RRSIG("a.miek.nl.	1800	IN	RRSIG	A 8 3 1800 20160426031301 20160327031301 12051 miek.nl. lxLotCjWZ3kihTxk="),
			test.CNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl."),
			test.RRSIG("www.miek.nl. 1800	RRSIG	CNAME 8 3 1800 20160426031301 20160327031301 12051 miek.nl.  NVZmMJaypS+wDL2Lar4Zw1zF"),
		},
		Ns: auth,
	},
	{
		// NoData
		Qname: "a.miek.nl.", Qtype: dns.TypeSRV, Do: true,
		Ns: []dns.RR{
			test.NSEC("a.miek.nl.	14400	IN	NSEC	archive.miek.nl. A AAAA RRSIG NSEC"),
			test.RRSIG("a.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160426031301 20160327031301 12051 miek.nl. GqnF6cutipmSHEao="),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "b.miek.nl.", Qtype: dns.TypeA, Do: true,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.NSEC("archive.miek.nl.	14400	IN	NSEC	go.dns.miek.nl. CNAME RRSIG NSEC"),
			test.RRSIG("archive.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160426031301 20160327031301 12051 miek.nl. jEpx8lcp4do5fWXg="),
			test.NSEC("miek.nl.	14400	IN	NSEC	a.miek.nl. A NS SOA MX AAAA RRSIG NSEC DNSKEY"),
			test.RRSIG("miek.nl.	14400	IN	RRSIG	NSEC 8 2 14400 20160426031301 20160327031301 12051 miek.nl. mFfc3r/9PSC1H6oSpdC"),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "b.blaat.miek.nl.", Qtype: dns.TypeA, Do: true,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.NSEC("archive.miek.nl.	14400	IN	NSEC	go.dns.miek.nl. CNAME RRSIG NSEC"),
			test.RRSIG("archive.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160426031301 20160327031301 12051 miek.nl. jEpx8lcp4do5fWXg="),
			test.NSEC("miek.nl.	14400	IN	NSEC	a.miek.nl. A NS SOA MX AAAA RRSIG NSEC DNSKEY"),
			test.RRSIG("miek.nl.	14400	IN	RRSIG	NSEC 8 2 14400 20160426031301 20160327031301 12051 miek.nl. mFfc3r/9PSC1H6oSpdC"),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "b.a.miek.nl.", Qtype: dns.TypeA, Do: true,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			// dedupped NSEC, because 1 nsec tells all
			test.NSEC("a.miek.nl.	14400	IN	NSEC	archive.miek.nl. A AAAA RRSIG NSEC"),
			test.RRSIG("a.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160426031301 20160327031301 12051 miek.nl. GqnF6cut/RRGPQ1QGQE1ipmSHEao="),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
}

var auth = []dns.RR{
	test.NS("miek.nl.	1800	IN	NS	ext.ns.whyscream.net."),
	test.NS("miek.nl.	1800	IN	NS	linode.atoom.net."),
	test.NS("miek.nl.	1800	IN	NS	ns-ext.nlnetlabs.nl."),
	test.NS("miek.nl.	1800	IN	NS	omval.tednet.nl."),
	test.RRSIG("miek.nl.	1800	IN	RRSIG	NS 8 2 1800 20160426031301 20160327031301 12051 miek.nl. ZLtsQhwazbqSpztFoR1Vxs="),
}

func TestLookupDNSSEC(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekNLSigned), testzone, "stdin", 0)
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range dnssecTestCases {
		m := tc.Msg()

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := fm.ServeDNS(ctx, rec, m)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		resp := rec.Msg
		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}
}

func BenchmarkFileLookupDNSSEC(b *testing.B) {
	zone, err := Parse(strings.NewReader(dbMiekNLSigned), testzone, "stdin", 0)
	if err != nil {
		return
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	tc := test.Case{
		Qname: "b.miek.nl.", Qtype: dns.TypeA, Do: true,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.NSEC("archive.miek.nl.	14400	IN	NSEC	go.dns.miek.nl. CNAME RRSIG NSEC"),
			test.RRSIG("archive.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160426031301 20160327031301 12051 miek.nl. jEpx8lcp4do5fWXg="),
			test.NSEC("miek.nl.	14400	IN	NSEC	a.miek.nl. A NS SOA MX AAAA RRSIG NSEC DNSKEY"),
			test.RRSIG("miek.nl.	14400	IN	RRSIG	NSEC 8 2 14400 20160426031301 20160327031301 12051 miek.nl. mFfc3r/9PSC1H6oSpdC"),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160426031301 20160327031301 12051 miek.nl. FIrzy07acBbtyQczy1dc="),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	}

	m := tc.Msg()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fm.ServeDNS(ctx, rec, m)
	}
}

const dbMiekNLSigned = `
; File written on Sun Mar 27 04:13:01 2016
; dnssec_signzone version 9.10.3-P4-Ubuntu
miek.nl.		1800	IN SOA	linode.atoom.net. miek.miek.nl. (
					1459051981 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)
			1800	RRSIG	SOA 8 2 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					FIrzy07acBzrf6kNW13Ypmq/ahojoMqOj0qJ
					ixTevTvwOEcVuw9GlJoYIHTYg+hm1sZHtx9K
					RiVmYsm8SHKsJA1WzixtT4K7vQvM+T+qbeOJ
					xA6YTivKUcGRWRXQlOTUAlHS/KqBEfmxKgRS
					68G4oOEClFDSJKh7RbtyQczy1dc= )
			1800	NS	ext.ns.whyscream.net.
			1800	NS	omval.tednet.nl.
			1800	NS	linode.atoom.net.
			1800	NS	ns-ext.nlnetlabs.nl.
			1800	RRSIG	NS 8 2 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					ZLtsQhwaz+CwrgzgFiEAqbqS/JH65MYjziA3
					6EXwlGDy41lcfGm71PpxA7cDzFhWNkJNk4QF
					q48wtpP4IGPPpHbnJHKDUXj6se7S+ylAGbS+
					VgVJ4YaVcE6xA9ZVhVpz8CSSjeH34vmqq9xj
					zmFjofuDvraZflHfNpztFoR1Vxs= )
			1800	A	139.162.196.78
			1800	RRSIG	A 8 2 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					hl+6Q075tsCkxIqbop8zZ6U8rlFvooz7Izzx
					MgCZYVLcg75El28EXKIhBfRb1dPaKbd+v+AD
					wrJMHL131pY5sU2Ly05K+7CqmmyaXgDaVsKS
					rSw/TbhGDIItBemeseeuXGAKAbY2+gE7kNN9
					mZoQ9hRB3SrxE2jhctv66DzYYQQ= )
			1800	MX	1 aspmx.l.google.com.
			1800	MX	5 alt1.aspmx.l.google.com.
			1800	MX	5 alt2.aspmx.l.google.com.
			1800	MX	10 aspmx2.googlemail.com.
			1800	MX	10 aspmx3.googlemail.com.
			1800	RRSIG	MX 8 2 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					kLqG+iOrKSzms1H9Et9me8Zts1rbyeCFSVQD
					G9is/u6ec3Lqg2vwJddf/yRsjVpVgadWSAkc
					GSDuD2dK8oBeP24axWc3Z1OY2gdMI7w+PKWT
					Z+pjHVjbjM47Ii/a6jk5SYeOwpGMsdEwhtTP
					vk2O2WGljifqV3uE7GshF5WNR10= )
			1800	AAAA	2a01:7e00::f03c:91ff:fef1:6735
			1800	RRSIG	AAAA 8 2 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					SsRTHytW4YTAuHovHQgfIMhNwMtMp4gaAU/Z
					lgTO+IkBb9y9F8uHrf25gG6RqA1bnGV/gezV
					NU5negXm50bf1BNcyn3aCwEbA0rCGYIL+nLJ
					szlBVbBu6me/Ym9bbJlfgfHRDfsVy2ZkNL+B
					jfNQtGCSDoJwshjcqJlfIVSardo= )
			14400	NSEC	a.miek.nl. A NS SOA MX AAAA RRSIG NSEC DNSKEY
			14400	RRSIG	NSEC 8 2 14400 (
					20160426031301 20160327031301 12051 miek.nl.
					mFfc3r/9PSC1H6oSpdC+FDy/Iu02W2Tf0x+b
					n6Lpe1gCC1uvcSUrrmBNlyAWRr5Zm+ZXssEb
					cKddRGiu/5sf0bUWrs4tqokL/HUl10X/sBxb
					HfwNAeD7R7+CkpMv67li5AhsDgmQzpX2r3P6
					/6oZyLvODGobysbmzeWM6ckE8IE= )
			1800	DNSKEY	256 3 8 (
					AwEAAcNEU67LJI5GEgF9QLNqLO1SMq1EdoQ6
					E9f85ha0k0ewQGCblyW2836GiVsm6k8Kr5EC
					IoMJ6fZWf3CQSQ9ycWfTyOHfmI3eQ/1Covhb
					2y4bAmL/07PhrL7ozWBW3wBfM335Ft9xjtXH
					Py7ztCbV9qZ4TVDTW/Iyg0PiwgoXVesz
					) ; ZSK; alg = RSASHA256; key id = 12051
			1800	DNSKEY	257 3 8 (
					AwEAAcWdjBl4W4wh/hPxMDcBytmNCvEngIgB
					9Ut3C2+QI0oVz78/WK9KPoQF7B74JQ/mjO4f
					vIncBmPp6mFNxs9/WQX0IXf7oKviEVOXLjct
					R4D1KQLX0wprvtUIsQFIGdXaO6suTT5eDbSd
					6tTwu5xIkGkDmQhhH8OQydoEuCwV245ZwF/8
					AIsqBYDNQtQ6zhd6jDC+uZJXg/9LuPOxFHbi
					MTjp6j3CCW0kHbfM/YHZErWWtjPj3U3Z7knQ
					SIm5PO5FRKBEYDdr5UxWJ/1/20SrzI3iztvP
					wHDsA2rdHm/4YRzq7CvG4N0t9ac/T0a0Sxba
					/BUX2UVPWaIVBdTRBtgHi0s=
					) ; KSK; alg = RSASHA256; key id = 33694
			1800	RRSIG	DNSKEY 8 2 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					o/D6o8+/bNGQyyRvwZ2hM0BJ+3HirvNjZoko
					yGhGe9sPSrYU39WF3JVIQvNJFK6W3/iwlKir
					TPOeYlN6QilnztFq1vpCxwj2kxJaIJhZecig
					LsKxY/fOHwZlIbBLZZadQG6JoGRLHnImSzpf
					xtyVaXQtfnJFC07HHt9np3kICfE= )
			1800	RRSIG	DNSKEY 8 2 1800 (
					20160426031301 20160327031301 33694 miek.nl.
					Ak/mbbQVQV+nUgw5Sw/c+TSoYqIwbLARzuNE
					QJvJNoRR4tKVOY6qSxQv+j5S7vzyORZ+yeDp
					NlEa1T9kxZVBMABoOtLX5kRqZncgijuH8fxb
					L57Sv2IzINI9+DOcy9Q9p9ygtwYzQKrYoNi1
					0hwHi6emGkVG2gGghruMinwOJASGgQy487Yd
					eIpcEKJRw73nxd2le/4/Vafy+mBpKWOczfYi
					5m9MSSxcK56NFYjPG7TvdIw0m70F/smY9KBP
					pGWEdzRQDlqfZ4fpDaTAFGyRX0mPFzMbs1DD
					3hQ4LHUSi/NgQakdH9eF42EVEDeL4cI69K98
					6NNk6X9TRslO694HKw== )
a.miek.nl.		1800	IN A	139.162.196.78
			1800	RRSIG	A 8 3 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					lxLotCjWZ3kikNNcePu6HOCqMHDINKFRJRD8
					laz2KQ9DKtgXPdnRw5RJvVITSj8GUVzw1ec1
					CYVEKu/eMw/rc953Zns528QBypGPeMNLe2vu
					C6a6UhZnGHA48dSd9EX33eSJs0MP9xsC9csv
					LGdzYmv++eslkKxkhSOk2j/hTxk= )
			1800	AAAA	2a01:7e00::f03c:91ff:fef1:6735
			1800	RRSIG	AAAA 8 3 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					ji3QMlaUzlK85ppB5Pc+y2WnfqOi6qrm6dm1
					bXgsEov/5UV1Lmcv8+Y5NBbTbBlXGlWcpqNp
					uWpf9z3lbguDWznpnasN2MM8t7yxo/Cr7WRf
					QCzui7ewpWiA5hq7j0kVbM4nnDc6cO+U93hO
					mMhVbeVI70HM2m0HaHkziEyzVZk= )
			14400	NSEC	archive.miek.nl. A AAAA RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20160426031301 20160327031301 12051 miek.nl.
					GqnF6cut/KCxbnJj27MCjjVGkjObV0hLhHOP
					E1/GXAUTEKG6BWxJq8hidS3p/yrOmP5PEL9T
					4FjBp0/REdVmGpuLaiHyMselES82p/uMMdY5
					QqRM6LHhZdO1zsRbyzOZbm5MsW6GR7K2kHlX
					9TdBIULiRRGPQ1QGQE1ipmSHEao= )
archive.miek.nl.	1800	IN CNAME a.miek.nl.
			1800	RRSIG	CNAME 8 3 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					s4zVJiDrVuUiUFr8CNQLuXYYfpqpl8rovL50
					BYsub/xK756NENiOTAOjYH6KYg7RSzsygJjV
					YQwXolZly2/KXAr48SCtxzkGFxLexxiKcFaj
					vm7ZDl7Btoa5l68qmBcxOX5E/W0IKITi4PNK
					mhBs7dlaf0IbPGNgMxae72RosxM= )
			14400	NSEC	go.dns.miek.nl. CNAME RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20160426031301 20160327031301 12051 miek.nl.
					jEp7LsoK++/PRFh2HieLzasA1jXBpp90NyDf
					RfpfOxdM69yRKfvXMc2bazIiMuDhxht79dGI
					Gj02cn1cvX60SlaHkeFtqTdJcHdK9rbI65EK
					YHFZFzGh9XVnuMJKpUsm/xS1dnUSAnXN8q+0
					xBlUDlQpsAFv/cx8lcp4do5fWXg= )
go.dns.miek.nl.		1800	IN TXT	"Hello!"
			1800	RRSIG	TXT 8 4 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					O0uo1NsXTq2TTfgOmGbHQQEchrcpllaDAMMX
					dTDizw3t+vZ5SR32qJ8W7y6VXLgUqJgcdRxS
					Fou1pp+t5juRZSQ0LKgxMpZAgHorkzPvRf1b
					E9eBKrDSuLGagsQRwHeldFGFgsXtCbf07vVH
					zoKR8ynuG4/cAoY0JzMhCts+56U= )
			14400	NSEC	www.miek.nl. TXT RRSIG NSEC
			14400	RRSIG	NSEC 8 4 14400 (
					20160426031301 20160327031301 12051 miek.nl.
					BW6qo7kYe3Z+Y0ebaVTWTy1c3bpdf8WUEoXq
					WDQxLDEj2fFiuEBDaSN5lTWRg3wj8kZmr6Uk
					LvX0P29lbATFarIgkyiAdbOEdaf88nMfqBW8
					z2T5xrPQcN0F13uehmv395yAJs4tebRxErMl
					KdkVF0dskaDvw8Wo3YgjHUf6TXM= )
www.miek.nl.		1800	IN CNAME a.miek.nl.
			1800	RRSIG	CNAME 8 3 1800 (
					20160426031301 20160327031301 12051 miek.nl.
					MiQQh2lScoNiNVZmMJaypS+wDL2Lar4Zw1zF
					Uo4tL16BfQOt7yl8gXdAH2JMFqoKAoIdM2K6
					XwFOwKTOGSW0oNCOcaE7ts+1Z1U0H3O2tHfq
					FAzfg1s9pQ5zxk8J/bJgkVIkw2/cyB0y1/PK
					EmIqvChBSb4NchTuMCSqo63LJM8= )
			14400	NSEC	miek.nl. CNAME RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20160426031301 20160327031301 12051 miek.nl.
					OPPZ8iaUPrVKEP4cqeCiiv1WLRAY30GRIhc/
					me0gBwFkbmTEnvB+rUp831OJZDZBNKv4QdZj
					Uyc26wKUOQeUyMJqv4IRDgxH7nq9GB5JRjYZ
					IVxtGD1aqWLXz+8aMaf9ARJjtYUd3K4lt8Wz
					LbJSo5Wdq7GOWqhgkY5n3XD0/FA= )`
