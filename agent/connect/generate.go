package connect

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// GeneratePrivateKey returns a new private key
func GeneratePrivateKey() (string, error) {
	var pk *ecdsa.PrivateKey

	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("error generating private key: %s", err)
	}

	bs, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return "", fmt.Errorf("error generating private key: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), nil
}
