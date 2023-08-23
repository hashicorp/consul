// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tlsutil

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"testing"
	"time"

	"strings"

	"github.com/stretchr/testify/require"
)

func TestSerialNumber(t *testing.T) {
	n1, err := GenerateSerialNumber()
	require.Nil(t, err)

	n2, err := GenerateSerialNumber()
	require.Nil(t, err)
	require.NotEqual(t, n1, n2)

	n3, err := GenerateSerialNumber()
	require.Nil(t, err)
	require.NotEqual(t, n1, n3)
	require.NotEqual(t, n2, n3)

}

func TestGeneratePrivateKey(t *testing.T) {
	t.Parallel()
	_, p, err := GeneratePrivateKey()
	require.Nil(t, err)
	require.NotEmpty(t, p)
	require.Contains(t, p, "BEGIN EC PRIVATE KEY")
	require.Contains(t, p, "END EC PRIVATE KEY")

	block, _ := pem.Decode([]byte(p))
	pk, err := x509.ParseECPrivateKey(block.Bytes)

	require.Nil(t, err)
	require.NotNil(t, pk)
	require.Equal(t, 256, pk.Params().BitSize)
}

type TestSigner struct {
	public interface{}
}

func (s *TestSigner) Public() crypto.PublicKey {
	return s.public
}

func (s *TestSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return []byte{}, nil
}

func TestGenerateCA(t *testing.T) {
	t.Run("no signer", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{Signer: &TestSigner{}})
		require.Error(t, err)
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("wrong key", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{Signer: &TestSigner{public: &rsa.PublicKey{}}})
		require.Error(t, err)
		require.Empty(t, ca)
		require.Empty(t, pk)
	})

	t.Run("valid key", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{})
		require.Nil(t, err)
		require.NotEmpty(t, ca)
		require.NotEmpty(t, pk)

		cert, err := parseCert(ca)
		require.Nil(t, err)
		require.True(t, strings.HasPrefix(cert.Subject.CommonName, "Consul Agent CA"))
		require.Equal(t, true, cert.IsCA)
		require.Equal(t, true, cert.BasicConstraintsValid)

		require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
		require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 365), time.Minute)

		require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
	})

	t.Run("RSA key", func(t *testing.T) {
		ca, pk, err := GenerateCA(CAOpts{})
		require.NoError(t, err)
		require.NotEmpty(t, ca)
		require.NotEmpty(t, pk)

		cert, err := parseCert(ca)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(cert.Subject.CommonName, "Consul Agent CA"))
		require.Equal(t, true, cert.IsCA)
		require.Equal(t, true, cert.BasicConstraintsValid)

		require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
		require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 365), time.Minute)

		require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
	})
}

func TestGenerateCert(t *testing.T) {
	t.Parallel()
	signer, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)
	ca, _, err := GenerateCA(CAOpts{Signer: signer})
	require.Nil(t, err)

	DNSNames := []string{"server.dc1.consul"}
	IPAddresses := []net.IP{net.ParseIP("123.234.243.213")}
	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	name := "Cert Name"
	certificate, pk, err := GenerateCert(CertOpts{
		Signer: signer, CA: ca, Name: name, Days: 365,
		DNSNames: DNSNames, IPAddresses: IPAddresses, ExtKeyUsage: extKeyUsage,
	})
	require.Nil(t, err)
	require.NotEmpty(t, certificate)
	require.NotEmpty(t, pk)

	cert, err := parseCert(certificate)
	require.Nil(t, err)
	require.Equal(t, name, cert.Subject.CommonName)
	require.Equal(t, true, cert.BasicConstraintsValid)
	signee, err := ParseSigner(pk)
	require.Nil(t, err)
	certID, err := keyID(signee.Public())
	require.Nil(t, err)
	require.Equal(t, certID, cert.SubjectKeyId)
	caID, err := keyID(signer.Public())
	require.Nil(t, err)
	require.Equal(t, caID, cert.AuthorityKeyId)
	require.Contains(t, cert.Issuer.CommonName, "Consul Agent CA")
	require.Equal(t, false, cert.IsCA)

	require.WithinDuration(t, cert.NotBefore, time.Now(), time.Minute)
	require.WithinDuration(t, cert.NotAfter, time.Now().AddDate(0, 0, 365), time.Minute)

	require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment, cert.KeyUsage)
	require.Equal(t, extKeyUsage, cert.ExtKeyUsage)

	// https://github.com/golang/go/blob/10538a8f9e2e718a47633ac5a6e90415a2c3f5f1/src/crypto/x509/verify.go#L414
	require.Equal(t, DNSNames, cert.DNSNames)
	require.True(t, IPAddresses[0].Equal(cert.IPAddresses[0]))
}
