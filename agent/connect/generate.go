package connect

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultPrivateKeyType      = "ec"
	DefaultPrivateKeyBits      = 256
	DefaultIntermediateCertTTL = 24 * 365 * time.Hour
)

func pemEncode(value []byte, blockType string) (string, error) {
	var buf bytes.Buffer

	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: value}); err != nil {
		return "", fmt.Errorf("error encoding value %v: %s", blockType, err)
	}
	return buf.String(), nil
}

func generateRSAKey(keyBits int) (crypto.Signer, string, error) {
	var pk *rsa.PrivateKey

	pk, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, "", fmt.Errorf("error generating RSA private key: %s", err)
	}

	bs := x509.MarshalPKCS1PrivateKey(pk)
	pemBlock, err := pemEncode(bs, "RSA PRIVATE KEY")
	if err != nil {
		return nil, "", err
	}

	return pk, pemBlock, nil
}

func generateECDSAKey(keyBits int) (crypto.Signer, string, error) {
	var pk *ecdsa.PrivateKey
	var curve elliptic.Curve

	switch keyBits {
	case 224:
		curve = elliptic.P224()
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		return nil, "", fmt.Errorf("error generating ECDSA private key: unknown curve length %d", keyBits)
	}

	pk, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("error generating ECDSA private key: %s", err)
	}

	bs, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return nil, "", fmt.Errorf("error marshaling ECDSA private key: %s", err)
	}

	pemBlock, err := pemEncode(bs, "EC PRIVATE KEY")
	if err != nil {
		return nil, "", err
	}

	return pk, pemBlock, nil
}

// GeneratePrivateKey generates a new Private key
func GeneratePrivateKeyWithConfig(keyType string, keyBits int) (crypto.Signer, string, error) {
	switch strings.ToLower(keyType) {
	case "rsa":
		return generateRSAKey(keyBits)
	case "ec":
		return generateECDSAKey(keyBits)
	default:
		return nil, "", fmt.Errorf("unknown private key type requested: %s", keyType)
	}
}

func GeneratePrivateKey() (crypto.Signer, string, error) {
	// TODO: find any calls to this func, replace with calls to GeneratePrivateKeyWithConfig()
	// using prefs `private_key_type` and `private_key_bits`
	return GeneratePrivateKeyWithConfig(DefaultPrivateKeyType, DefaultPrivateKeyBits)
}
