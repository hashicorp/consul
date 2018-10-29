package connect

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"time"
)

// GenerateSerialNumber returns random bigint generated with crypto/rand
func GenerateSerialNumber() (*big.Int, error) {
	l := new(big.Int).Lsh(big.NewInt(1), 128)
	s, err := rand.Int(rand.Reader, l)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GeneratePrivateKey generates a new ecdsa private key
func GeneratePrivateKey() (crypto.Signer, string, error) {
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("error generating private key: %s", err)
	}

	bs, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return nil, "", fmt.Errorf("error generating private key: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: bs})
	if err != nil {
		return nil, "", fmt.Errorf("error encoding private key: %s", err)
	}

	return pk, buf.String(), nil
}

// GenerateCA generates a new CA
func GenerateCA(signer crypto.Signer, sn *big.Int, days int, uris []*url.URL) (string, error) {
	keyID, err := KeyId(signer.Public())
	if err != nil {
		return "", err
	}

	name := fmt.Sprintf("Consul CA %d", sn)

	// Create the CA cert
	template := x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			Country:       []string{"US"},
			PostalCode:    []string{"94105"},
			Province:      []string{"CA"},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"101 Second Street"},
			Organization:  []string{"HashiCorp Inc."},
			CommonName:    name,
		},
		URIs:                  uris,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		NotAfter:              time.Now().AddDate(0, 0, days),
		NotBefore:             time.Now(),
		AuthorityKeyId:        keyID,
		SubjectKeyId:          keyID,
	}

	bs, err := x509.CreateCertificate(
		rand.Reader, &template, &template, signer.Public(), signer)
	if err != nil {
		return "", fmt.Errorf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), nil
}

// GenerateCert generates a new certificate
func GenerateCert(signer crypto.Signer, ca string, sn *big.Int, name string, days int, DNSNames []string, IPAddresses []net.IP, extKeyUsage []x509.ExtKeyUsage) (string, string, error) {
	parent, err := ParseCert(ca)
	if err != nil {
		return "", "", err
	}

	signee, pk, err := GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}

	keyID, err := KeyId(signee.Public())
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: name},
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           extKeyUsage,
		IsCA:                  false,
		NotAfter:              time.Now().AddDate(0, 0, days),
		NotBefore:             time.Now(),
		SubjectKeyId:          keyID,
		DNSNames:              DNSNames,
		IPAddresses:           IPAddresses,
	}

	bs, err := x509.CreateCertificate(rand.Reader, &template, parent, signee.Public(), signer)
	if err != nil {
		return "", "", err
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), pk, nil
}

func GenerateCSR() (string, error) {
	return "", nil
}
