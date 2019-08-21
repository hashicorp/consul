package file

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

var entTestCases = []test.Case{
	{
		Qname: "b.c.miek.nl.", Qtype: dns.TypeA,
		Ns: []dns.RR{
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "b.c.miek.nl.", Qtype: dns.TypeA, Do: true,
		Ns: []dns.RR{
			test.NSEC("a.miek.nl.	14400	IN	NSEC	a.b.c.miek.nl. A RRSIG NSEC"),
			test.RRSIG("a.miek.nl.	14400	IN	RRSIG	NSEC 8 3 14400 20160502144311 20160402144311 12051 miek.nl. d5XZEy6SUpq98ZKUlzqhAfkLI9pQPc="),
			test.RRSIG("miek.nl.	1800	IN	RRSIG	SOA 8 2 1800 20160502144311 20160402144311 12051 miek.nl. KegoBxA3Tbrhlc4cEdkRiteIkOfsq"),
			test.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
}

func TestLookupEnt(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbMiekENTNL), testzone, "stdin", 0)
	if err != nil {
		t.Fatalf("Expect no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{testzone: zone}, Names: []string{testzone}}}
	ctx := context.TODO()

	for _, tc := range entTestCases {
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

const dbMiekENTNL = `; File written on Sat Apr  2 16:43:11 2016
; dnssec_signzone version 9.10.3-P4-Ubuntu
miek.nl.		1800	IN SOA	linode.atoom.net. miek.miek.nl. (
					1282630057 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)
			1800	RRSIG	SOA 8 2 1800 (
					20160502144311 20160402144311 12051 miek.nl.
					KegoBxA3Tbrhlc4cEdkRiteIkOfsqD4oCLLM
					ISJ5bChWy00LGHUlAnHVu5Ti96hUjVNmGSxa
					xtGSuAAMFCr52W8pAB8LBIlu9B6QZUPHMccr
					SuzxAX3ioawk2uTjm+k8AGPT4RoQdXemGLAp
					zJTASolTVmeMTh5J0sZTZJrtvZ0= )
			1800	NS	linode.atoom.net.
			1800	RRSIG	NS 8 2 1800 (
					20160502144311 20160402144311 12051 miek.nl.
					m0cOHL6Rre/0jZPXe+0IUjs/8AFASRCvDbSx
					ZQsRDSlZgS6RoMP3OC77cnrKDVlfZ2Vhq3Ce
					nYPoGe0/atB92XXsilmstx4HTSU64gsV9iLN
					Xkzk36617t7zGOl/qumqfaUXeA9tihItzEim
					6SGnufVZI4o8xeyaVCNDDuN0bvY= )
			14400	NSEC	a.miek.nl. NS SOA RRSIG NSEC DNSKEY
			14400	RRSIG	NSEC 8 2 14400 (
					20160502144311 20160402144311 12051 miek.nl.
					BCWVgwxWrs4tBjS9QXKkftCUbiLi40NyH1yA
					nbFy1wCKQ2jDH00810+ia4b66QrjlAKgxE9z
					9U7MKSMV86sNkyAtlCi+2OnjtWF6sxPdJO7k
					CHeg46XBjrQuiJRY8CneQX56+IEPdufLeqPR
					l+ocBQ2UkGhXmQdWp3CFDn2/eqU= )
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
					20160502144311 20160402144311 12051 miek.nl.
					YNpi1jRDQKpnsQEjIjxqy+kJGaYnV16e8Iug
					40c82y4pee7kIojFUllSKP44qiJpCArxF557
					tfjfwBd6c4hkqCScGPZXJ06LMyG4u//rhVMh
					4hyKcxzQFKxmrFlj3oQGksCI8lxGX6RxiZuR
					qv2ol2lUWrqetpAL+Zzwt71884E= )
			1800	RRSIG	DNSKEY 8 2 1800 (
					20160502144311 20160402144311 33694 miek.nl.
					jKpLDEeyadgM0wDgzEk6sBBdWr2/aCrkAOU/
					w6dYIafN98f21oIYQfscV1gc7CTsA0vwzzUu
					x0QgwxoNLMvSxxjOiW/2MzF8eozczImeCWbl
					ad/pVCYH6Jn5UBrZ5RCWMVcs2RP5KDXWeXKs
					jEN/0EmQg5qNd4zqtlPIQinA9I1HquJAnS56
					pFvYyGIbZmGEbhR18sXVBeTWYr+zOMHn2quX
					0kkrx2udz+sPg7i4yRsLdhw138gPRy1qvbaC
					8ELs1xo1mC9pTlDOhz24Q3iXpVAU1lXLYOh9
					nUP1/4UvZEYXHBUQk/XPRciojniWjAF825x3
					QoSivMHblBwRdAKJSg== )
a.miek.nl.		1800	IN A	127.0.0.1
			1800	RRSIG	A 8 3 1800 (
					20160502144311 20160402144311 12051 miek.nl.
					lUOYdSxScjyYz+Ebc+nb6iTNgCohqj7K+Dat
					97KE7haV2nP3LxdYuDCJYZpeyhsXDLHd4bFI
					bInYPwJiC6DUCxPCuCWy0KYlZOWW8KCLX3Ia
					BOPQbvIwLsJhnX+/tyMD9mXortoqATO79/6p
					nNxvFeM8pFDwaih17fXMuFR/BsI= )
			14400	NSEC	a.b.c.miek.nl. A RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20160502144311 20160402144311 12051 miek.nl.
					d5XZEy6SUp+TPRJQED+0R65zf2Yeo/1dlEA2
					jYYvkXGSHXke4sg9nH8U3nr1rLcuqA1DsQgH
					uMIjdENvXuZ+WCSwvIbhC+JEI6AyQ6Gfaf/D
					I3mfu60C730IRByTrKM5C2rt11lwRQlbdaUY
					h23/nn/q98ZKUlzqhAfkLI9pQPc= )
a.b.c.miek.nl.		1800	IN A	127.0.0.1
			1800	RRSIG	A 8 5 1800 (
					20160502144311 20160402144311 12051 miek.nl.
					FwgU5+fFD4hEebco3gvKQt3PXfY+dcOJr8dl
					Ky4WLsONIdhP+4e9oprPisSLxImErY21BcrW
					xzu1IZrYDsS8XBVV44lBx5WXEKvAOrUcut/S
					OWhFZW7ncdIQCp32ZBIatiLRJEqXUjx+guHs
					noFLiHix35wJWsRKwjGLIhH1fbs= )
			14400	NSEC	miek.nl. A RRSIG NSEC
			14400	RRSIG	NSEC 8 5 14400 (
					20160502144311 20160402144311 12051 miek.nl.
					lXgOqm9/jRRYvaG5jC1CDvTtGYxMroTzf4t4
					jeYGb60+qI0q9sHQKfAJvoQ5o8o1qfR7OuiF
					f544ipYT9eTcJRyGAOoJ37yMie7ZIoVJ91tB
					r8YdzZ9Q6x3v1cbwTaQiacwhPZhGYOw63qIs
					q5IQErIPos2sNk+y9D8BEce2DO4= )`
