package file

const dbDnssexNLSigned = `
; File written on Tue Mar 29 21:02:24 2016
; dnssec_signzone version 9.10.3-P4-Ubuntu
dnssex.nl.		1800	IN SOA	linode.atoom.net. miek.miek.nl. (
					1459281744 ; serial
					14400      ; refresh (4 hours)
					3600       ; retry (1 hour)
					604800     ; expire (1 week)
					14400      ; minimum (4 hours)
					)
			1800	RRSIG	SOA 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					CA/Y3m9hCOiKC/8ieSOv8SeP964BUdG/8MC3
					WtKljUosK9Z9bBGrVizDjjqgq++lyH8BZJcT
					aabAsERs4xj5PRtcxicwQXZACX5VYjXHQeZm
					CyytFU5wq2gcXSmvUH86zZzftx3RGPvn1aOo
					TlcvoC3iF8fYUCpROlUS0YR8Cdw= )
			1800	NS	omval.tednet.nl.
			1800	NS	linode.atoom.net.
			1800	NS	ns-ext.nlnetlabs.nl.
			1800	RRSIG	NS 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					dLIeEvP86jj5nd3orv9bH7hTvkblF4Na0sbl
					k6fJA6ha+FPN1d6Pig3NNEEVQ/+wlOp/JTs2
					v07L7roEEUCbBprI8gMSld2gFDwNLW3DAB4M
					WD/oayYdAnumekcLzhgvWixTABjWAGRTGQsP
					sVDFXsGMf9TGGC9FEomgkCVeNC0= )
			1800	A	139.162.196.78
			1800	RRSIG	A 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					LKJKLzPiSEDWOLAag2YpfD5EJCuDcEAJu+FZ
					Xy+4VyOv9YvRHCTL4vbrevOo5+XymY2RxU1q
					j+6leR/Fe7nlreSj2wzAAk2bIYn4m6r7hqeO
					aKZsUFfpX8cNcFtGEywfHndCPELbRxFeEziP
					utqHFLPNMX5nYCpS28w4oJ5sAnM= )
			1800	TXT	"Doing It Safe Is Better"
			1800	RRSIG	TXT 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					f6S+DUfJK1UYdOb3AHgUXzFTTtu+yLp/Fv7S
					Hv0CAGhXAVw+nBbK719igFvBtObS33WKwzxD
					1pQNMaJcS6zeevtD+4PKB1KDC4fyJffeEZT6
					E30jGR8Y29/xA+Fa4lqDNnj9zP3b8TiABCle
					ascY5abkgWCALLocFAzFJQ/27YQ= )
			1800	AAAA	2a01:7e00::f03c:91ff:fef1:6735
			1800	RRSIG	AAAA 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					PWcPSawEUBAfCuv0liEOQ8RYe7tfNW4rubIJ
					LE+dbrub1DUer3cWrDoCYFtOufvcbkYJQ2CQ
					AGjJmAQ5J2aqYDOPMrKa615V0KT3ifbZJcGC
					gkIic4U/EXjaQpRoLdDzR9MyVXOmbA6sKYzj
					ju1cNkLqM8D7Uunjl4pIr6rdSFo= )
			14400	NSEC	*.dnssex.nl. A NS SOA TXT AAAA RRSIG NSEC DNSKEY
			14400	RRSIG	NSEC 8 2 14400 (
					20160428190224 20160329190224 14460 dnssex.nl.
					oIvM6JZIlNc1aNKGTxv58ApSnDr1nDPPgnD9
					9oJZRIn7eb5WnpeDz2H3z5+x6Bhlp5hJJaUp
					KJ3Ss6Jg/IDnrmIvKmgq6L6gHj1Y1IiHmmU8
					VeZTRzdTsDx/27OsN23roIvsytjveNSEMfIm
					iLZ23x5kg1kBdJ9p3xjYHm5lR+8= )
			1800	DNSKEY	256 3 8 (
					AwEAAazSO6uvLPEVknDA8yxjFe8nnAMU7txp
					wb19k55hQ81WV3G4bpBM1NdN6sbYHrkXaTNx
					2bQWAkvX6pz0XFx3z/MPhW+vkakIWFYpyQ7R
					AT5LIJfToVfiCDiyhhF0zVobKBInO9eoGjd9
					BAW3TUt+LmNAO/Ak5D5BX7R3CuA7v9k7
					) ; ZSK; alg = RSASHA256; key id = 14460
			1800	DNSKEY	257 3 8 (
					AwEAAbyeaV9zg0IqdtgYoqK5jJ239anzwG2i
					gvH1DxSazLyaoNvEkCIvPgMLW/JWfy7Z1mQp
					SMy9DtzL5pzRyQgw7kIeXLbi6jufUFd9pxN+
					xnzKLf9mY5AcnGToTrbSL+jnMT67wG+c34+Q
					PeVfucHNUePBxsbz2+4xbXiViSQyCQGv
					) ; KSK; alg = RSASHA256; key id = 18772
			1800	RRSIG	DNSKEY 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					cFSFtJE+DBGNxb52AweFaVHBe5Ue5MDpqNdC
					TIneUnEhP2m+vK4zJ/TraK0WdQFpsX63pod8
					PZ9y03vHUfewivyonCCBD3DcNdoU9subhN22
					tez9Ct8Z5/9E4RAz7orXal4M1VUEhRcXSEH8
					SJW20mfVsqJAiKqqNeGB/pAj23I= )
			1800	RRSIG	DNSKEY 8 2 1800 (
					20160428190224 20160329190224 18772 dnssex.nl.
					oiiwo/7NYacePqohEp50261elhm6Dieh4j2S
					VZGAHU5gqLIQeW9CxKJKtSCkBVgUo4cvO4Rn
					2tzArAuclDvBrMXRIoct8u7f96moeFE+x5FI
					DYqICiV6k449ljj9o4t/5G7q2CRsEfxZKpTI
					A/L0+uDk0RwVVzL45+TnilcsmZs= )
*.dnssex.nl.		1800	IN TXT	"Doing It Safe Is Better"
			1800	RRSIG	TXT 8 2 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					FUZSTyvZfeuuOpCmNzVKOfITRHJ6/ygjmnnb
					XGBxVUyQjoLuYXwD5XqZWGw4iKH6QeSDfGCx
					4MPqA4qQmW7Wwth7mat9yMfA4+p2sO84bysl
					7/BG9+W2G+q1uQiM9bX9V42P2X/XuW5Y/t9Y
					8u1sljQ7D8WwS6naH/vbaJxnDBw= )
			14400	NSEC	a.dnssex.nl. TXT RRSIG NSEC
			14400	RRSIG	NSEC 8 2 14400 (
					20160428190224 20160329190224 14460 dnssex.nl.
					os6INm6q2eXknD5z8TpfbK00uxVbQefMvHcR
					/RNX/kh0xXvzAaaDOV+Ge/Ko+2dXnKP+J1LY
					G9ffXNpdbaQy5ygzH5F041GJst4566GdG/jt
					7Z7vLHYxEBTpZfxo+PLsXQXH3VTemZyuWyDf
					qJzafXJVH1F0nDrcXmMlR6jlBHA= )
www.dnssex.nl.		1800	IN CNAME a.dnssex.nl.
			1800	RRSIG	CNAME 8 3 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					Omv42q/uVvdNsWQoSrQ6m6w6U7r7Abga7uF4
					25b3gZlse0C+WyMyGFMGUbapQm7azvBpreeo
					uKJHjzd+ufoG+Oul6vU9vyoj+ejgHzGLGbJQ
					HftfP+UqP5SWvAaipP/LULTWKPuiBcLDLiBI
					PGTfsq0DB6R+qCDTV0fNnkgxEBQ= )
			14400	NSEC	dnssex.nl. CNAME RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20160428190224 20160329190224 14460 dnssex.nl.
					TBN3ddfZW+kC84/g3QlNNJMeLZoyCalPQylt
					KXXLPGuxfGpl3RYRY8KaHbP+5a8MnHjqjuMB
					Lofb7yKMFxpSzMh8E36vnOqry1mvkSakNj9y
					9jM8PwDjcpYUwn/ql76MsmNgEV5CLeQ7lyH4
					AOrL79yOSQVI3JHJIjKSiz88iSw= )
a.dnssex.nl.		1800	IN A	139.162.196.78
			1800	RRSIG	A 8 3 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					OXHpFj9nSpKi5yA/ULH7MOpGAWfyJ2yC/2xa
					Pw0fqSY4QvcRt+V3adcFA4H9+P1b32GpxEjB
					lXmCJID+H4lYkhUR4r4IOZBVtKG2SJEBZXip
					pH00UkOIBiXxbGzfX8VL04v2G/YxUgLW57kA
					aknaeTOkJsO20Y+8wmR9EtzaRFI= )
			1800	AAAA	2a01:7e00::f03c:91ff:fef1:6735
			1800	RRSIG	AAAA 8 3 1800 (
					20160428190224 20160329190224 14460 dnssex.nl.
					jrepc/VnRzJypnrG0WDEqaAr3HMjWrPxJNX0
					86gbFjZG07QxBmrA1rj0jM9YEWTjjyWb2tT7
					lQhzKDYX/0XdOVUeeOM4FoSks80V+pWR8fvj
					AZ5HmX69g36tLosMDKNR4lXcrpv89QovG4Hr
					/r58fxEKEFJqrLDjMo6aOrg+uKA= )
			14400	NSEC	www.dnssex.nl. A AAAA RRSIG NSEC
			14400	RRSIG	NSEC 8 3 14400 (
					20160428190224 20160329190224 14460 dnssex.nl.
					S+UM62wXRNNFN3QDWK5YFWUbHBXC4aqaqinZ
					A2ZDeC+IQgyw7vazPz7cLI5T0YXXks0HTMlr
					soEjKnnRZsqSO9EuUavPNE1hh11Jjm0fB+5+
					+Uro0EmA5Dhgc0Z2VpbXVQEhNDf/pI1gem15
					RffN2tBYNykZn4Has2ySgRaaRYQ= )`
