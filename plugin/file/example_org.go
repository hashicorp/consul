package file

// exampleOrgSigned is a fake signed example.org zone with two delegations,
// one signed (with DSs) and one "normal".
const exampleOrgSigned = `
example.org.		1800	IN SOA	a.iana-servers.net. devnull.example.org. (
					1282630057 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)
			1800	RRSIG	SOA 13 2 1800 (
					20161129153240 20161030153240 49035 example.org.
					GVnMpFmN+6PDdgCtlYDEYBsnBNDgYmEJNvos
					Bk9+PNTPNWNst+BXCpDadTeqRwrr1RHEAQ7j
					YWzNwqn81pN+IA== )
			1800	NS	a.iana-servers.net.
			1800	NS	b.iana-servers.net.
			1800	RRSIG	NS 13 2 1800 (
					20161129153240 20161030153240 49035 example.org.
					llrHoIuwjnbo28LOt4p5zWAs98XGqrXicKVI
					Qxyaf/ORM8boJvW2XrKr3nj6Y8FKMhzd287D
					5PBzVCL6MZyjQg== )
			14400	NSEC	a.example.org. NS SOA RRSIG NSEC DNSKEY
			14400	RRSIG	NSEC 13 2 14400 (
					20161129153240 20161030153240 49035 example.org.
					BQROf1swrmYi3GqpP5M/h5vTB8jmJ/RFnlaX
					7fjxvV7aMvXCsr3ekWeB2S7L6wWFihDYcKJg
					9BxVPqxzBKeaqg== )
			1800	DNSKEY	256 3 13 (
					UNTqlHbC51EbXuY0rshW19Iz8SkCuGVS+L0e
					bQj53dvtNlaKfWmtTauC797FoyVLbQwoMy/P
					G68SXgLCx8g+9g==
					) ; ZSK; alg = ECDSAP256SHA256; key id = 49035
			1800	RRSIG	DNSKEY 13 2 1800 (
					20161129153240 20161030153240 49035 example.org.
					LnLHyqYJaCMOt7EHB4GZxzAzWLwEGCTFiEhC
					jj1X1VuQSjJcN42Zd3yF+jihSW6huknrig0Z
					Mqv0FM6mJ/qPKg== )
a.delegated.example.org. 1800	IN A	139.162.196.78
			1800	TXT	"obscured"
			1800	AAAA	2a01:7e00::f03c:91ff:fef1:6735
archive.example.org.	1800	IN CNAME a.example.org.
			1800	RRSIG	CNAME 13 3 1800 (
					20161129153240 20161030153240 49035 example.org.
					SDFW1z/PN9knzH8BwBvmWK0qdIwMVtGrMgRw
					7lgy4utRrdrRdCSLZy3xpkmkh1wehuGc4R0S
					05Z3DPhB0Fg5BA== )
			14400	NSEC	delegated.example.org. CNAME RRSIG NSEC
			14400	RRSIG	NSEC 13 3 14400 (
					20161129153240 20161030153240 49035 example.org.
					DQqLSVNl8F6v1K09wRU6/M6hbHy2VUddnOwn
					JusJjMlrAOmoOctCZ/N/BwqCXXBA+d9yFGdH
					knYumXp+BVPBAQ== )
www.example.org.	1800	IN CNAME a.example.org.
			1800	RRSIG	CNAME 13 3 1800 (
					20161129153240 20161030153240 49035 example.org.
					adzujOxCV0uBV4OayPGfR11iWBLiiSAnZB1R
					slmhBFaDKOKSNYijGtiVPeaF+EuZs63pzd4y
					6Nm2Iq9cQhAwAA== )
			14400	NSEC	example.org. CNAME RRSIG NSEC
			14400	RRSIG	NSEC 13 3 14400 (
					20161129153240 20161030153240 49035 example.org.
					jy3f96GZGBaRuQQjuqsoP1YN8ObZF37o+WkV
					PL7TruzI7iNl0AjrUDy9FplP8Mqk/HWyvlPe
					N3cU+W8NYlfDDQ== )
a.example.org.		1800	IN A	139.162.196.78
			1800	RRSIG	A 13 3 1800 (
					20161129153240 20161030153240 49035 example.org.
					41jFz0Dr8tZBN4Kv25S5dD4vTmviFiLx7xSA
					qMIuLFm0qibKL07perKpxqgLqM0H1wreT4xz
					I9Y4Dgp1nsOuMA== )
			1800	AAAA	2a01:7e00::f03c:91ff:fef1:6735
			1800	RRSIG	AAAA 13 3 1800 (
					20161129153240 20161030153240 49035 example.org.
					brHizDxYCxCHrSKIu+J+XQbodRcb7KNRdN4q
					VOWw8wHqeBsFNRzvFF6jwPQYphGP7kZh1KAb
					VuY5ZVVhM2kHjw== )
			14400	NSEC	archive.example.org. A AAAA RRSIG NSEC
			14400	RRSIG	NSEC 13 3 14400 (
					20161129153240 20161030153240 49035 example.org.
					zIenVlg5ScLr157EWigrTGUgrv7W/1s49Fic
					i2k+OVjZfT50zw+q5X6DPKkzfAiUhIuqs53r
					hZUzZwV/1Wew9Q== )
delegated.example.org.	1800	IN NS	a.delegated.example.org.
			1800	IN NS	ns-ext.nlnetlabs.nl.
			1800	DS	10056 5 1 (
					EE72CABD1927759CDDA92A10DBF431504B9E
					1F13 )
			1800	DS	10056 5 2 (
					E4B05F87725FA86D9A64F1E53C3D0E625094
					6599DFE639C45955B0ED416CDDFA )
			1800	RRSIG	DS 13 3 1800 (
					20161129153240 20161030153240 49035 example.org.
					rlNNzcUmtbjLSl02ZzQGUbWX75yCUx0Mug1j
					HtKVqRq1hpPE2S3863tIWSlz+W9wz4o19OI4
					jbznKKqk+DGKog== )
			14400	NSEC	sub.example.org. NS DS RRSIG NSEC
			14400	RRSIG	NSEC 13 3 14400 (
					20161129153240 20161030153240 49035 example.org.
					lNQ5kRTB26yvZU5bFn84LYFCjwWTmBcRCDbD
					cqWZvCSw4LFOcqbz1/wJKIRjIXIqnWIrfIHe
					fZ9QD5xZsrPgUQ== )
sub.example.org.	1800	IN NS	sub1.example.net.
			1800	IN NS	sub2.example.net.
			14400	NSEC	www.example.org. NS RRSIG NSEC
			14400	RRSIG	NSEC 13 3 14400 (
					20161129153240 20161030153240 49035 example.org.
					VYjahdV+TTkA3RBdnUI0hwXDm6U5k/weeZZr
					ix1znORpOELbeLBMJW56cnaG+LGwOQfw9qqj
					bOuULDst84s4+g== )
`
