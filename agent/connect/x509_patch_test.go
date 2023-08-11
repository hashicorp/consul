// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
)

var testPrivateKey_x509 *rsa.PrivateKey

func TestX509_EmptySubject(t *testing.T) {
	// NOTE: this test is lifted straight out of the stdlib with no changes. to
	// show that the cert-only workflow is fine.

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{"example.com"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &testPrivateKey_x509.PublicKey, testPrivateKey_x509)
	if err != nil {
		t.Fatalf("failed to create certificate: %s", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %s", err)
	}

	for _, ext := range cert.Extensions {
		if ext.Id.Equal(x509_oidExtensionSubjectAltName) {
			if !ext.Critical {
				t.Fatal("SAN extension is not critical")
			}
			return
		}
	}

	t.Fatal("SAN extension is missing")
}

func TestX509_EmptySubjectInCSR(t *testing.T) {
	// NOTE: the CSR-only workflow is flawed so we hack around it

	for _, tc := range []struct {
		name           string
		hack           bool
		expectCritical bool
	}{
		{name: "unmodified stdlib",
			hack:           false,
			expectCritical: false,
		},
		{name: "hacked stdlib",
			hack:           true,
			expectCritical: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			template := x509.CertificateRequest{
				DNSNames: []string{"example.com"},
			}
			if tc.hack {
				HackSANExtensionForCSR(&template)
			}

			derBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, testPrivateKey_x509)
			if err != nil {
				t.Fatalf("failed to create certificate request: %s", err)
			}

			csr, err := x509.ParseCertificateRequest(derBytes)
			if err != nil {
				t.Fatalf("failed to parse certificate request: %s", err)
			}

			for _, ext := range csr.Extensions {
				if ext.Id.Equal(x509_oidExtensionSubjectAltName) {
					if tc.expectCritical {
						if !ext.Critical {
							t.Fatal("SAN extension is not critical")
						}
					} else {
						if ext.Critical {
							t.Fatal("SAN extension is critical now; maybe we don't need the hack anymore with this version of Go?")
						}
					}
					return
				}
			}

			t.Fatal("SAN extension is missing")
		})
	}
}

func init() {
	block, _ := pem.Decode([]byte(pemPrivateKey_x509))

	var err error
	testPrivateKey_x509, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
}

var pemPrivateKey_x509 = `
-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCxoeCUW5KJxNPxMp+KmCxKLc1Zv9Ny+4CFqcUXVUYH69L3mQ7v
IWrJ9GBfcaA7BPQqUlWxWM+OCEQZH1EZNIuqRMNQVuIGCbz5UQ8w6tS0gcgdeGX7
J7jgCQ4RK3F/PuCM38QBLaHx988qG8NMc6VKErBjctCXFHQt14lerd5KpQIDAQAB
AoGAYrf6Hbk+mT5AI33k2Jt1kcweodBP7UkExkPxeuQzRVe0KVJw0EkcFhywKpr1
V5eLMrILWcJnpyHE5slWwtFHBG6a5fLaNtsBBtcAIfqTQ0Vfj5c6SzVaJv0Z5rOd
7gQF6isy3t3w9IF3We9wXQKzT6q5ypPGdm6fciKQ8RnzREkCQQDZwppKATqQ41/R
vhSj90fFifrGE6aVKC1hgSpxGQa4oIdsYYHwMzyhBmWW9Xv/R+fPyr8ZwPxp2c12
33QwOLPLAkEA0NNUb+z4ebVVHyvSwF5jhfJxigim+s49KuzJ1+A2RaSApGyBZiwS
rWvWkB471POAKUYt5ykIWVZ83zcceQiNTwJBAMJUFQZX5GDqWFc/zwGoKkeR49Yi
MTXIvf7Wmv6E++eFcnT461FlGAUHRV+bQQXGsItR/opIG7mGogIkVXa3E1MCQARX
AAA7eoZ9AEHflUeuLn9QJI/r0hyQQLEtrpwv6rDT1GCWaLII5HJ6NUFVf4TTcqxo
6vdM4QGKTJoO+SaCyP0CQFdpcxSAuzpFcKv0IlJ8XzS/cy+mweCMwyJ1PFEc4FX6
wg/HcAJWY60xZTJDFN+Qfx8ZQvBEin6c2/h+zZi5IVY=
-----END RSA PRIVATE KEY-----
`
