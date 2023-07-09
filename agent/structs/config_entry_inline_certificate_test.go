// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import "testing"

const (
	// generated via openssl req -x509 -sha256 -days 1825 -newkey rsa:2048 -keyout private.key -out certificate.crt
	validPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0wzZeonUklhOvJ0AxcdDdCTiMwR9tsm/6IGcw9Jm50xVY+qg
5GFg1RWrQaODq7Gjqd/JDUAwtTBnQMs1yt6nbsHe2QhbD4XeqtZ+6fTv1ZpG3k8F
eB/M01xFqovczRV/ie77wd4vqoPD+AcfD8NDAFJt3htwUgGIqkQHP329Sh3TtLga
9ZMCs1MoTT+POYGUPL8bwt9R6ClNrucbH4Bs6OnX2ZFbKF75O9OHKNxWTmpDSodv
OFbFyKps3BfnPuF0Z6mj5M5yZeCjmtfS25PrsM3pMBGK5YHb0MlFfZIrIGboMbrz
9F/BMQJ64pMe43KwqHvTnbKWhp6PzLhEkPGLnwIDAQABAoIBADBEJAiONPszDu67
yU1yAM8zEDgysr127liyK7PtDnOfVXgAVMNmMcsJpZzhVF+TxKY487YAFCOb6kE7
OBYpTYla9SgVbR3js8TGQUgoKCFlowd8cvfB7gn4dEZIrjqIzB4zdYgk1Cne8JZs
qoHkWhJcx5ugEtPuXd7yp+WxT/T+6uOro06scp67NhP5t9yoAGFv5Vdb577RuzRo
Wkd9higQ9A20+GtjCY0EYxdgRviWvW7mM5/F+Lzcaui86ME+ga754gX8zgW3+NJ5
LMsz5OLSnh291Uyjmr77HWBv/xvpq01Fls0LyJcgxFVZuJs5GQz+l3otSqv4FTP6
Ua9w/YECgYEA8To3dgUK1QhzX5rwhWtlst3pItGTvmEdNzXmjgSylu7uKM13i+xg
llhp2uXrOEtuL+xtBZdeFNaijusbyqjg0xj6e4o31c19okuuDkJD5/sfQq22bvrn
gVJMGuESprIiPePrEyrXCHOdxH6eDgR2dIzAeO5vz0nnKGFAWrJJbvECgYEA3/mJ
eacXOJznw4Sa8jGWS2FtZLKxDHph7uDKMJmuG0ukb3aHJ9dMHrPleCLo8mhpoObA
hueoIbIP7swGrQx79+nZbnQpF6rMp6FAU5bF3gSrj1eWbaeh8pn9mrv4hal9USmn
orTbXMxDp3XSh7voR8Fqy5tMQqwZ+Lz74ccbw48CgYEA5cEhGdNrocPOv3x/IVRN
JLOfXX5nTaiJfxBja1imEIO5ajtoZWjaBdhn2gmqo4+UfyicHfsxrH9RjPX5HmkC
2Yys5gWbcJOr2Wxjd0k+DDFucL+rRsDKxq1vtxov/X0kh/YQ68ydynr0BTbjq04s
1I1KtOPEspYdCKS3+qpcrsECgYBtvYeVesBO9do9G0kMKC26y4bdEwzaz1ASykNn
IrWDHEH6dznr1HqwhHaHsZsvwucWdlmZAAKKWAOkfoU63uYS55qomvPTa9WQwNqS
2koi6Wjh+Al1uvAHvVncKgOwAgar8Nv5ReJBirgPYhSAexpppiRclL/93vNuw7Iq
wvMgkwKBgQC5wnb6SUUrzzKKSRgyusHM/XrjiKgVKq7lvFE9/iJkcw+BEXpjjbEe
RyD0a7PRtCfR39SMVrZp4KXVNNK5ln0WhuLvraMDwOpH9JDWHQiAhuJ3ooSwBylK
+QCLjyOtWAGZAIBRJyb1txfTXZ++dldkOjBi3bmEiadOa48ksvDsNQ==
-----END RSA PRIVATE KEY-----`
	validCertificate = `-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQC6cMRYsE+ahDANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAkxBMQ0wCwYDVQQKDARUZXN0MQ0wCwYD
VQQLDARTdHViMRwwGgYDVQQDDBNob3N0LmNvbnN1bC5leGFtcGxlMB4XDTIzMDIx
NzAyMTA1MloXDTI4MDIxNjAyMTA1MlowYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgM
AkNBMQswCQYDVQQHDAJMQTENMAsGA1UECgwEVGVzdDENMAsGA1UECwwEU3R1YjEc
MBoGA1UEAwwTaG9zdC5jb25zdWwuZXhhbXBsZTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBANMM2XqJ1JJYTrydAMXHQ3Qk4jMEfbbJv+iBnMPSZudMVWPq
oORhYNUVq0Gjg6uxo6nfyQ1AMLUwZ0DLNcrep27B3tkIWw+F3qrWfun079WaRt5P
BXgfzNNcRaqL3M0Vf4nu+8HeL6qDw/gHHw/DQwBSbd4bcFIBiKpEBz99vUod07S4
GvWTArNTKE0/jzmBlDy/G8LfUegpTa7nGx+AbOjp19mRWyhe+TvThyjcVk5qQ0qH
bzhWxciqbNwX5z7hdGepo+TOcmXgo5rX0tuT67DN6TARiuWB29DJRX2SKyBm6DG6
8/RfwTECeuKTHuNysKh7052yloaej8y4RJDxi58CAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAHF10odRNJ7TKvcD2JPtR8wMacfldSiPcQnn+rhMUyBaKOoSrALxOev+N
L8N+RtEV+KXkyBkvT71OZzEpY9ROwqOQ/acnMdbfG0IBPbg3c/7WDD2sjcdr1zvc
U3T7WJ7G3guZ5aWCuAGgOyT6ZW8nrDa4yFbKZ1PCJkvUQ2ttO1lXmyGPM533Y2pi
SeXP6LL7z5VNqYO3oz5IJEstt10IKxdmb2gKFhHjgEmHN2gFL0jaPi4mjjaINrxq
MdqcM9IzLr26AjZ45NuI9BCcZWO1mraaQTOIb3QL5LyqaC7CRJXLYPSGARthyDhq
J3TrQE3YVrL4D9xnklT86WDnZKApJg==
-----END CERTIFICATE-----`
	mismatchedCertificate = `-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQC2H6+PYz23xDANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAkxBMQ0wCwYDVQQKDARUZXN0MQwwCgYD
VQQLDANGb28xHTAbBgNVBAMMFG90aGVyLmNvbnN1bC5leGFtcGxlMB4XDTIzMDIx
NzAyMTM0OVoXDTI4MDIxNjAyMTM0OVowYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgM
AkNBMQswCQYDVQQHDAJMQTENMAsGA1UECgwEVGVzdDEMMAoGA1UECwwDRm9vMR0w
GwYDVQQDDBRvdGhlci5jb25zdWwuZXhhbXBsZTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAO0IH/dzmWJaTPVL32xQVHivrnQk38vskW0ymILYuaismUMJ
0+xrcaTcVljU+3nKhmSW9wcYSFY02GcGWAdcw8x8xO801cna020T+DIWiYaljXT3
agrbYfULF9q+ihT6IL1D2mFa0AW1x6Bk1XAmZRSTpRBhp7iFNnCXGRK8sSSr95ge
DxaRyj/2F8t6kG+ANPkRBiPd2rRgsYQjuTLuZYBvseeJygnSF8ty1QMg6koz7kdN
bPon3Q5GFH71WNwzm9G3DWjMIu+dhpHz7rsbCnhwLB5lh1jsZBYkAMt3kiyY0g4I
ReuiVWesMe+AMG/DQZvZ5mE252QFJ92dLTeo5RcCAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAijm6blixjl+pMRAj7EajoPjU+GqhooZayJrvdwvofwcPxQYpkPuh7Uc6
l2z494b75cRzMw7wS+iW/ad8NYrfw1JwHMsUfncxs5LDO5GsKl9Krg/39goDl3wC
ywTcl00y+FMYfldNPjKDLunENmn+yPa2pKuBVQ0yOKALp+oUeJFVzRNPV5fohlBi
HjypkO0KaVmCG6P01cqCgVkNzxnX9qQYP3YXX1yt5iOcI7QcoOa5WnRhOuD8WqJ1
v3AZGYNvKyXf9E5nD0y2Cmz6t1awjFjzMlXMx6AdHrjWqxtHhYQ1xz4P4NfzK27m
cCtURSzXMgcrSeZLepBfdICf+0/0+Q==
-----END CERTIFICATE-----`
	emptyCNPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAx95Opa6t4lGEpiTUogEBptqOdam2ch4BHQGhNhX/MrDwwuZQ
httBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2jQlhqTodElkbsd5vWY8R/bxJWQSo
NvVE12TlzECxGpJEiHt4W0r8pGffk+rvpljiUyCfnT1kGF3znOSjK1hRMTn6RKWC
yYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409g9X5VU88/Bmmrz4cMyxce86Kc2ug
5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftrXOvuCbO5IBRHMOBHiHTZ4rtGuhMa
Ir21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+WmQIDAQABAoIBACYvceUzp2MK4gYA
GWPOP2uKbBdM0l+hHeNV0WAM+dHMfmMuL4pkT36ucqt0ySOLjw6rQyOZG5nmA6t9
sv0g4ae2eCMlyDIeNi1Yavu4Wt6YX4cTXbQKThm83C6W2X9THKbauBbxD621bsDK
7PhiGPN60yPue7YwFQAPqqD4YaK+s22HFIzk9gwM/rkvAUNwRv7SyHMiFe4Igc1C
Eev7iHWzvj5Heoz6XfF+XNF9DU+TieSUAdjd56VyUb8XL4+uBTOhHwLiXvAmfaMR
HvpcxeKnYZusS6NaOxcUHiJnsLNWrxmJj9WEGgQzuLxcLjTe4vVmELVZD8t3QUKj
PAxu8tUCgYEA7KIWVn9dfVpokReorFym+J8FzLwSktP9RZYEMonJo00i8aii3K9s
u/aSwRWQSCzmON1ZcxZzWhwQF9usz6kGCk//9+4hlVW90GtNK0RD+j7sp4aT2JI8
9eLEjTG+xSXa7XWe98QncjjL9lu/yrRncSTxHs13q/XP198nn2aYuQ8CgYEA2Dnt
sRBzv0fFEvzzFv7G/5f85mouN38TUYvxNRTjBLCXl9DeKjDkOVZ2b6qlfQnYXIru
H+W+v+AZEb6fySXc8FRab7lkgTMrwE+aeI4rkW7asVwtclv01QJ5wMnyT84AgDD/
Dgt/RThFaHgtU9TW5GOZveL+l9fVPn7vKFdTJdcCgYEArJ99zjHxwJ1whNAOk1av
09UmRPm6TvRo4heTDk8oEoIWCNatoHI0z1YMLuENNSnT9Q280FFDayvnrY/qnD7A
kktT/sjwJOG8q8trKzIMqQS4XWm2dxoPcIyyOBJfCbEY6XuRsUuePxwh5qF942EB
yS9a2s6nC4Ix0lgPrqAIr48CgYBgS/Q6riwOXSU8nqCYdiEkBYlhCJrKpnJxF9T1
ofa0yPzKZP/8ZEfP7VzTwHjxJehQ1qLUW9pG08P2biH1UEKEWdzo8vT6wVJT1F/k
HtTycR8+a+Hlk2SHVRHqNUYQGpuIe8mrdJ1as4Pd0d/F/P0zO9Rlh+mAsGPM8HUM
T0+9gwKBgHDoerX7NTskg0H0t8O+iSMevdxpEWp34ZYa9gHiftTQGyrRgERCa7Gj
nZPAxKb2JoWyfnu3v7G5gZ8fhDFsiOxLbZv6UZJBbUIh1MjJISpXrForDrC2QNLX
kHrHfwBFDB3KMudhQknsJzEJKCL/KmFH6o0MvsoaT9yzEl3K+ah/
-----END RSA PRIVATE KEY-----`
	emptyCNCertificate = `-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCQMDsYO8FrPjANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJV
UzAeFw0yMjEyMjAxNzUwMjVaFw0yNzEyMTkxNzUwMjVaMA0xCzAJBgNVBAYTAlVT
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAx95Opa6t4lGEpiTUogEB
ptqOdam2ch4BHQGhNhX/MrDwwuZQhttBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2
jQlhqTodElkbsd5vWY8R/bxJWQSoNvVE12TlzECxGpJEiHt4W0r8pGffk+rvplji
UyCfnT1kGF3znOSjK1hRMTn6RKWCyYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409
g9X5VU88/Bmmrz4cMyxce86Kc2ug5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftr
XOvuCbO5IBRHMOBHiHTZ4rtGuhMaIr21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+W
mQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBfCqoUIdPf/HGSbOorPyZWbyizNtHJ
GL7x9cAeIYxpI5Y/WcO1o5v94lvrgm3FNfJoGKbV66+JxOge731FrfMpHplhar1Z
RahYIzNLRBTLrwadLAZkApUpZvB8qDK4knsTWFYujNsylCww2A6ajzIMFNU4GkUK
NtyHRuD+KYRmjXtyX1yHNqfGN3vOQmwavHq2R8wHYuBSc6LAHHV9vG+j0VsgMELO
qwxn8SmLkSKbf2+MsQVzLCXXN5u+D8Yv+4py+oKP4EQ5aFZuDEx+r/G/31rTthww
AAJAMaoXmoYVdgXV+CPuBb2M4XCpuzLu3bcA2PXm5ipSyIgntMKwXV7r
-----END CERTIFICATE-----`
	tooShortPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCtmK1VjmXJ7vm4CZkkOSjc+kjGNMlyce5rXxwlDRz9LcGGc3Tg
kwUJesyBpDtxLLVHXQIPr5mWYbX/W/ezQ9sntxrATbDek8pBgoOlARebwkD2ivVW
BWfVhlryVihWlXApKiJ2n3i0m+OVtdrceC9Bv2hEMhYVOwzxtb3O0YFkbwIDAQAB
AoGAIxgnipFUEKPIRiVimUkY8ruCdNd9Fi7kNT6wEOl6v9A9PHIg4bm3Hfh+WYMb
JUEVkMzDuuoUEavFQE+WXt5L8oE1lEBmN2++FQsvllN+MRBTRg2sfw4mUWDI6S4r
h8+XNTzTIg2sUd2J3o2qNmQoOheYb+iuYDj76IFoEdwwZ0kCQQDYKKs5HAbnrLj1
UrOp8TyHdFf0YNw5tGdbNTbffq4rlBD6SW70+Sj624i2UqdnYwRiWzdXv3zN08aI
Vfoh2cGlAkEAzZe5B6BhiX/PcIYutMtuT3K+mysFNlowrutXWoQOpR7gGAkgEt6e
oCDgx1QJRjsp6NFQxKc6l034Hzs17gqJgwJAcu9U873aUg9+HTuHOoKB28haCCAE
mU46cr3d2oKCW7uUN3EaZXmid5iJneBfENMOfrnfuHGiC9NiShXlNWCS3QJAO5Ne
w83+1ahaxUGs4SkeExmuECrcPM7P0rBRxOIFmGWlDHIAgFdQYhiE6l34vghA8b1O
CV5oRRYL84jl7M/S3wJBALDfL5YXcc8P6scLJJ1biqhLYppvGN5CUwbsJsluvHCW
XCTVIbPOaS42A0xUfpoiTcdbNSFRvdCzPR5nsGy8Y7g=
-----END RSA PRIVATE KEY-----`
)

func TestInlineCertificate(t *testing.T) {
	t.Parallel()

	cases := map[string]configEntryTestcase{
		"invalid private key": {
			entry: &InlineCertificateConfigEntry{
				Kind:       InlineCertificate,
				Name:       "cert-one",
				PrivateKey: "foo",
			},
			validateErr: "failed to parse private key PEM",
		},
		"invalid certificate": {
			entry: &InlineCertificateConfigEntry{
				Kind:        InlineCertificate,
				Name:        "cert-two",
				PrivateKey:  validPrivateKey,
				Certificate: "foo",
			},
			validateErr: "failed to parse certificate PEM",
		},
		"invalid private key length": {
			entry: &InlineCertificateConfigEntry{
				Kind:        InlineCertificate,
				Name:        "cert-two",
				PrivateKey:  tooShortPrivateKey,
				Certificate: "foo",
			},
			validateErr: "key length must be at least 2048 bits",
		},
		"mismatched certificate": {
			entry: &InlineCertificateConfigEntry{
				Kind:        InlineCertificate,
				Name:        "cert-three",
				PrivateKey:  validPrivateKey,
				Certificate: mismatchedCertificate,
			},
			validateErr: "private key does not match public key",
		},
		"matched certificate": {
			entry: &InlineCertificateConfigEntry{
				Kind:        InlineCertificate,
				Name:        "cert-four",
				PrivateKey:  validPrivateKey,
				Certificate: validCertificate,
			},
		},
		"empty cn certificate": {
			entry: &InlineCertificateConfigEntry{
				Kind:        InlineCertificate,
				Name:        "cert-five",
				PrivateKey:  emptyCNPrivateKey,
				Certificate: emptyCNCertificate,
			},
			validateErr: "host \"\" must be a valid DNS hostname",
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}
