package structs

import "testing"

const (
	// generated via openssl req -x509 -sha256 -days 1825 -newkey rsa:2048 -keyout private.key -out certificate.crt
	validPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
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
	validCertificate = `-----BEGIN CERTIFICATE-----
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
	mismatchedCertificate = `-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQC49bq8e0QgLDANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJV
UzAeFw0yMjEyMjAxNzUyMzJaFw0yNzEyMTkxNzUyMzJaMA0xCzAJBgNVBAYTAlVT
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAk7Are9ulVDY0IqaG5Pt/
OVuS0kmDhgVUfQBM5JDGRfIsu1ebn68kn5JGCTQ+nC8nU9QXRJS7vG6As5GWm08W
FpkOyIbHLjOhWtYCYzQ+0R+sSSoMnczgl8l6wIUIkR3Vpoy6QUsSZbvo4/xDi3Uk
1CF+JMTM2oFDLD8PNrNzW/txRyTugK36W1G1ofUhvP6EHsTjmVcZwBcLOKToov6L
Ai758MLztl1/X/90DNdZwuHC9fGIgx52Ojz3+XIocXFttr+J8xZglMCtqL4n40bh
5b1DE+hC3NHQmA+7Chc99z28baj2cU1woNk/TO+ewqpyvj+WPWwGOQt3U63ZoPaw
yQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCMF3JlrDdcSv2KYrxEp1tWB/GglI8a
JiSvrf3hePaRz59099bg4DoHzTn0ptOcOPOO9epDPbCJrUqLuPlwvrQRvll6GaW1
y3TcbnE1AbwTAjbOTgpLhvuj6IVlyNNLoKbjZqs4A8N8i6UkQ7Y8qg77lwxD3QoH
pWLwGZKJifKPa7ObVWmKj727kbU59nA2Hx+Y4qa/MyiPWxJM9Y0JsFGxSBxp4kmQ
q4ikzSWaPv/TvtV+d4mO1H44aggdNMCYIQd/5BXQzG40l+ecHnBueJyG312ax/Zp
NsYUAKQT864cGlxrnWVgT4sW/tsl9Qen7g9iAdeBAPvLO7cQjAjtc7KZ
-----END CERTIFICATE-----`
)

func TestInlineCertificate(t *testing.T) {
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
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}
