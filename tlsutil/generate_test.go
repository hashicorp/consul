package tlsutil

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

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
	t.Parallel()
	sn, err := GenerateSerialNumber()
	require.Nil(t, err)
	var s crypto.Signer

	// test what happens without key
	s = &TestSigner{}
	ca, err := GenerateCA(s, sn, 0, nil)
	require.Error(t, err)
	require.Empty(t, ca)

	// test what happens with wrong key
	s = &TestSigner{public: &rsa.PublicKey{}}
	ca, err = GenerateCA(s, sn, 0, nil)
	require.Error(t, err)
	require.Empty(t, ca)

	// test what happens with correct key
	s, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)
	ca, err = GenerateCA(s, sn, 365, nil)
	require.Nil(t, err)
	require.NotEmpty(t, ca)

	cert, err := parseCert(ca)
	require.Nil(t, err)
	require.Equal(t, fmt.Sprintf("Consul Agent CA %d", sn), cert.Subject.CommonName)
	require.Equal(t, true, cert.IsCA)
	require.Equal(t, true, cert.BasicConstraintsValid)

	// format so that we don't take anything smaller than second into account.
	require.Equal(t, cert.NotBefore.Format(time.ANSIC), time.Now().UTC().Format(time.ANSIC))
	require.Equal(t, cert.NotAfter.Format(time.ANSIC), time.Now().AddDate(0, 0, 365).UTC().Format(time.ANSIC))

	require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
}

func TestGenerateCert(t *testing.T) {
	t.Parallel()
	sn, err := GenerateSerialNumber()
	require.Nil(t, err)
	signer, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)
	ca, err := GenerateCA(signer, sn, 365, nil)
	require.Nil(t, err)

	sn, err = GenerateSerialNumber()
	require.Nil(t, err)
	DNSNames := []string{"server.dc1.consul"}
	IPAddresses := []net.IP{net.ParseIP("123.234.243.213")}
	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	name := "Cert Name"
	certificate, pk, err := GenerateCert(signer, ca, sn, name, 365, DNSNames, IPAddresses, extKeyUsage)
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

	// format so that we don't take anything smaller than second into account.
	require.Equal(t, cert.NotBefore.Format(time.ANSIC), time.Now().UTC().Format(time.ANSIC))
	require.Equal(t, cert.NotAfter.Format(time.ANSIC), time.Now().AddDate(0, 0, 365).UTC().Format(time.ANSIC))

	require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment, cert.KeyUsage)
	require.Equal(t, extKeyUsage, cert.ExtKeyUsage)

	// https://github.com/golang/go/blob/10538a8f9e2e718a47633ac5a6e90415a2c3f5f1/src/crypto/x509/verify.go#L414
	require.Equal(t, DNSNames, cert.DNSNames)
	require.True(t, IPAddresses[0].Equal(cert.IPAddresses[0]))
}
