package file

import (
	"context"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

// another personal zone (helps in testing as my secondary is NSD, atoom = atom in English.
var atoomTestCases = []test.Case{
	{
		Qname: atoom, Qtype: dns.TypeNS, Do: true,
		Answer: []dns.RR{
			test.NS("atoom.net.		1800	IN	NS	linode.atoom.net."),
			test.NS("atoom.net.		1800	IN	NS	ns-ext.nlnetlabs.nl."),
			test.NS("atoom.net.		1800	IN	NS	omval.tednet.nl."),
			test.RRSIG("atoom.net.		1800	IN	RRSIG	NS 8 2 1800 20170112031301 20161213031301 53289 atoom.net. DLe+G1 jlw="),
		},
		Extra: []dns.RR{
			// test.OPT(4096, true), // added by server, not test in this unit test.
			test.A("linode.atoom.net.	1800	IN	A	176.58.119.54"),
			test.AAAA("linode.atoom.net.	1800	IN	AAAA	2a01:7e00::f03c:91ff:fe79:234c"),
			test.RRSIG("linode.atoom.net.	1800	IN	RRSIG	A 8 3 1800 20170112031301 20161213031301 53289 atoom.net. Z4Ka4OLDoyxj72CL vkI="),
			test.RRSIG("linode.atoom.net.	1800	IN	RRSIG	AAAA 8 3 1800 20170112031301 20161213031301 53289 atoom.net. l+9Qc914zFH/okG2fzJ1q olQ="),
		},
	},
}

func TestLookupGlue(t *testing.T) {
	zone, err := Parse(strings.NewReader(dbAtoomNetSigned), atoom, "stdin", 0)
	if err != nil {
		t.Fatalf("Expected no error when reading zone, got %q", err)
	}

	fm := File{Next: test.ErrorHandler(), Zones: Zones{Z: map[string]*Zone{atoom: zone}, Names: []string{atoom}}}
	ctx := context.TODO()

	for _, tc := range atoomTestCases {
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

const dbAtoomNetSigned = `
; File written on Tue Dec 13 04:13:01 2016
; dnssec_signzone version 9.10.3-P4-Debian
atoom.net.		1800	IN SOA	linode.atoom.net. miek.miek.nl. (
					1481602381 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)
			1800	RRSIG	SOA 8 2 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					GZ30uFuGATKzwHXgpEwK70qjdXSAqmbB5d4z
					e7WTibvJDPLa1ptZBI7Zuod2KMOkT1ocSvhL
					U7makhdv0BQx+5RSaP25mAmPIzfU7/T7R+DJ
					5q1GLlDSvOprfyMUlwOgZKZinesSdUa9gRmu
					8E+XnPNJ/jcTrGzzaDjn1/irrM0= )
			1800	NS	omval.tednet.nl.
			1800	NS	linode.atoom.net.
			1800	NS	ns-ext.nlnetlabs.nl.
			1800	RRSIG	NS 8 2 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					D8Sd9JpXIOxOrUF5Hi1ASutyQwP7JNu8XZxA
					rse86A6L01O8H8sCNib2VEoJjHuZ/dDEogng
					OgmfqeFy04cpSX19GAk3bkx8Lr6aEat3nqIC
					XA/xsCCfXy0NKZpI05zntHPbbP5tF/NvpE7n
					0+oLtlHSPEg1ZnEgwNoLe+G1jlw= )
			1800	A	176.58.119.54
			1800	RRSIG	A 8 2 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					mrjiUFNCqDgCW8TuhjzcMh0V841uC224QvwH
					0+OvYhcve9twbX3Y12PSFmz77Xz3Jg9WAj4I
					qhh3iHUac4dzUXyC702DT62yMF/9CMUO0+Ee
					b6wRtvPHr2Tt0i/xV/BTbArInIvurXJrvKvo
					LsZHOfsg7dZs6Mvdpe/CgwRExpk= )
			1800	AAAA	2a01:7e00::f03c:91ff:fe79:234c
			1800	RRSIG	AAAA 8 2 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					EkMxX2vUaP4h0qbWlHaT4yNhm8MrPMZTn/3R
					zNw+i3oF2cLMWKh6GCfuIX/x5ID706o8kfum
					bxTYwuTe1LJ+GoZHWEiH8VCa1laTlh8l3qSi
					PZKU8339rr5cCYluk6p9PbAuRkYYOEruNg42
					wPOx46dsAlvp2XpOaOeJtU64QGQ= )
			14400	NSEC	deb.atoom.net. A NS SOA AAAA RRSIG NSEC DNSKEY
			14400	RRSIG	NSEC 8 2 14400 (
					20170112031301 20161213031301 53289 atoom.net.
					P7Stx7lqRKl8tbTAAaJ0W6UhgJwZz3cjpM8z
					eplbhXEVohKtyJ9xgptKt1vreH6lkhzciar5
					EB9Nj0VOmcthiht/+As8aEKmf8UlcJ2EbLII
					NT7NUaasxsrLE2rjjX5mEtzOZ1uQAGiU8Hnk
					XdGweTgIVFuiCcMCgaKpC2TRrMw= )
			1800	DNSKEY	256 3 8 (
					AwEAAeDZTH9YT9qLMPlq4VrxX7H3GbWcqCrC
					tXc9RT/hf96GN+ttnnEQVaJY8Gbly3IZpYQW
					MwaCi0t30UULXE3s9FUQtl4AMbplyiz9EF8L
					/XoBS1yhGm5WV5u608ihoPaRkYNyVV3egb5Y
					hA5EXWy2vfsa1XWPpxvSAhlqM0YENtP3
					) ; ZSK; alg = RSASHA256; key id = 53289
			1800	DNSKEY	257 3 8 (
					AwEAAepN7Vo8enDCruVduVlGxTDIv7QG0wJQ
					fTL1hMy4k0Yf/7dXzrn5bZT4ytBvH1hoBImH
					mtTrQo6DQlBBVXDJXTyQjQozaHpN1HhTJJTz
					IXl8UrdbkLWvz6QSeJPmBBYQRAqylUA2KE29
					nxyiNboheDLiIWyQ7Q/Op7lYaKMdb555kQAs
					b/XT4Tb3/3BhAjcofNofNBjDjPq2i8pAo8HU
					5mW5/Pl+ZT/S0aqQPnCkHk/iofSRu3ZdBzkH
					54eoC+BdyXb7gTbPGRr+1gMbf/rzhRiZ4vnX
					NoEzGAXmorKzJHANNb6KQ/932V9UDHm9wbln
					6y3s7IBvsMX5KF8vo81Stkc=
					) ; KSK; alg = RSASHA256; key id = 19114
			1800	RRSIG	DNSKEY 8 2 1800 (
					20170112031301 20161213031301 19114 atoom.net.
					IEjViubKdef8RWB5bcnirqVcqDk16irkywJZ
					sBjMyNs03/a+sl0UHEGAB7qCC+Rn+RDaM5It
					WF+Gha6BwRIN9NuSg3BwB2h1nJtHw61pMVU9
					2j9Q3pq7X1xoTBAcwY95t5a1xlw0iTCaLu1L
					Iu/PbVp1gj1o8BF/PiYilvZJGUjaTgsi+YNi
					2kiWpp6afO78/W4nfVx+lQBmpyfX1lwL5PEC
					9f5PMbzRmOapvUBc2XdddGywLdmlNsLHimGV
					t7kkHZHOWQR1TvvMbU3dsC0bFCrBVGDhEuxC
					hATR+X5YV0AyDSyrew7fOGJKrapwMWS3yRLr
					FAt0Vcxno5lwQImbCQ== )
			1800	RRSIG	DNSKEY 8 2 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					sSxdgPT+gFZPN0ot6lZRGqOwvONUEsg0uEbf
					kh19JlWHu/qvq5HOOK2VOW/UnswpVmtpFk0W
					z/jiCNHifjpCCVn5tfCMZDLGekmPOjdobw24
					swBuGjnn0NHvxHoN6S+mb+AR6V/dLjquNUda
					yzBc2Ua+XtQ7SCLKIvEhcNg9H3o= )
deb.atoom.net.		1800	IN A	176.58.119.54
			1800	RRSIG	A 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					ZW7jm/VDa/I9DxWlE7Cm+HHymiVv4Wk5UGYI
					Uf/g0EfxLCBR6SwL5QKuV1z7xoWKaiNqqrmc
					gg35xgskKyS8QHgCCODhDzcIKe+MSsBXbY04
					AtrC5dV3JJQoA65Ng/48hwcyghAjXKrA2Yyq
					GXf2DSvWeIV9Jmk0CsOELP24dpk= )
			1800	TXT	"v=spf1 a ip6:2a01:7e00::f03c:91ff:fe79:234c ~all"
			1800	RRSIG	TXT 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					fpvVJ+Z6tzSd9yETn/PhLSCRISwRD1c3ET80
					8twnx3XfAPQfV2R8dw7pz8Vw4TSxvf19bAZc
					PWRjW682gb7gAxoJshCXBYabMfqExrBc9V1S
					ezwm3D93xNMyegxzHx2b/H8qp3ZWdsMLTvvN
					Azu7P4iyO+WRWT0R7bJGrdTwRz8= )
			1800	AAAA	2a01:7e00::f03c:91ff:fe79:234c
			1800	RRSIG	AAAA 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					aaPF6NqXfWamzi+xUDVeYa7StJUVM1tDsL34
					w5uozFRZ0f4K/Z88Kk5CgztxmtpNNKGdLWa0
					iryUJsbVWAbSQfrZNkNckBtczMNxGgjqn97A
					2//F6ajH/qrR3dWcCm+VJMgu3UPqAxLiCaYO
					GQUx6Y8JA1VIM/RJAM6BhgNxjD0= )
			14400	NSEC	lafhart.atoom.net. A TXT AAAA RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20170112031301 20161213031301 53289 atoom.net.
					1Llad64NDWcz8CyBu2TsyANrJ9Tpfm5257sY
					FPYF579p3c9Imwp9kYEO1zMEKgNoXBN/sQnd
					YCugq3r2GAI6bfJj8sV5bt6GKuZcGHMESug4
					uh2gU0NDcCA4GPdBYGdusePwV0RNpcRnVCFA
					fsACp+22j3uwRUbCh0re0ufbAs4= )
lafhart.atoom.net.	1800	IN A	178.79.160.171
			1800	RRSIG	A 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					fruP6cvMVICXEV8NcheS73NWLCEKlO1FgW6B
					35D2GhtfYZe+M23V5YBRtlVCCrAdS0etdCOf
					xH9yt3u2kVvDXuMRiQr1zJPRDEq3cScYumpd
					bOO8cjHiCic5lEcRVWNNHXyGtpqTvrp9CxOu
					IQw1WgAlZyKj43zGg3WZi6OTKLg= )
			14400	NSEC	linode.atoom.net. A RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20170112031301 20161213031301 53289 atoom.net.
					2AUWXbScL0jIJ7G6UsJAlUs+bgSprZ1zY6v/
					iVB5BAYwZD6pPky7LZdzvPEHh0aNLGIFbbU8
					SDJI7u/e4RUTlE+8yyjl6obZNfNKyJFqE5xN
					1BJ8sjFrVn6KaHIDKEOZunNb1MlMfCRkLg9O
					94zg04XEgVUfaYCPxvLs3fCEgzw= )
voordeur.atoom.net.	1800	IN A	77.249.87.46
			1800	RRSIG	A 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					SzJz0NaKLRA/lW4CxgMHgeuQLp5QqFEjQv3I
					zfPtY4joQsZn8RN8RLECcpcPKjbC8Dj6mxIJ
					dd2vwhsCVlZKMNcZUOfpB7eGx1TR9HnzMkY9
					OdTt30a9+tktagrJEoy31vAhj1hJqLbSgvOa
					pRr1P4ZpQ53/qH8JX/LOmqfWTdg= )
			14400	NSEC	www.atoom.net. A RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20170112031301 20161213031301 53289 atoom.net.
					CETJhUJy1rKjVj9wsW1549gth+/Z37//BI6S
					nxJ+2Oq63jEjlbznmyo5hvFW54DbVUod+cLo
					N9PdlNQDr1XsRBgWhkKW37RkuoRVEPwqRykv
					xzn9i7CgYKAAHFyWMGihBLkV9ByPp8GDR8Zr
					DEkrG3ErDlBcwi3FqGZFsSOW2xg= )
www.atoom.net.		1800	IN CNAME deb.atoom.net.
			1800	RRSIG	CNAME 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					1lhG6iTtbeesBCVOrA8a7+V2gogCuXzKgSi8
					6K0Pzq2CwqTScdNcZvcDOIbLq45Am5p09PIj
					lXnd2fw6WAxphwvRhmwCve3uTZMUt5STw7oi
					0rED7GMuFUSC/BX0XVly7NET3ECa1vaK6RhO
					hDSsKPWFI7to4d1z6tQ9j9Kvm4Y= )
			14400	NSEC	atoom.net. CNAME RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20170112031301 20161213031301 53289 atoom.net.
					CC4yCYP1q75/gTmPz+mVM6Lam2foPP5oTccY
					RtROuTkgbt8DtAoPe304vmNazWBlGidnWJeD
					YyAAe3znIHP0CgrxjD/hRL9FUzMnVrvB3mnx
					4W13wP1rE97RqJxV1kk22Wl3uCkVGy7LCjb0
					JLFvzCe2fuMe7YcTzI+t1rioTP0= )
linode.atoom.net.	1800	IN A	176.58.119.54
			1800	RRSIG	A 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					Z4Ka4OLDha4eQNWs3GtUd1Cumr48RUnH523I
					nZzGXtpQNou70qsm5Jt8n/HmsZ4L5DoxomRz
					rgZTGnrqj43+A16UUGfVEk6SfUUHOgxgspQW
					zoaqk5/5mQO1ROsLKY8RqaRqzvbToHvqeZEh
					VkTPVA02JK9UFlKqoyxj72CLvkI= )
			1800	AAAA	2a01:7e00::f03c:91ff:fe79:234c
			1800	RRSIG	AAAA 8 3 1800 (
					20170112031301 20161213031301 53289 atoom.net.
					l+9Qce/EQyKrTJVKLv7iatjuCO285ckd5Oie
					P2LzWVsL4tW04oHzieKZwIuNBRE+px8g5qrT
					LIK2TikCGL1xHAd7CT7gbCtDcZ7jHmSTmMTJ
					405nOV3G3xWelreLI5Fn5ck8noEsF64kiw1y
					XfkyQn2B914zFH/okG2fzJ1qolQ= )
			14400	NSEC	voordeur.atoom.net. A AAAA RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20170112031301 20161213031301 53289 atoom.net.
					Owzmz7QrVL2Gw2njEsUVEknMl2amx1HG9X3K
					tO+Ihyy4tApiUFxUjAu3P/30QdqbB85h7s//
					ipwX/AmQJNoxTScR3nHt9qDqJ044DPmiuh0l
					NuIjguyZRANApmKCTA6AoxXIUqToIIjfVzi/
					PxXE6T3YIPlK7Bxgv1lcCBJ1fmE= )`

const atoom = "atoom.net."
