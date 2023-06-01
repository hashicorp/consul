// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/hashicorp/consul/api"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

func ApplyDefaultProxySettings(c *api.Client) (bool, error) {
	req := &api.ProxyConfigEntry{
		Name: "global",
		Kind: "proxy-defaults",
		Config: map[string]any{
			"protocol": "tcp",
		},
	}
	ok, _, err := c.ConfigEntries().Set(req, &api.WriteOptions{})
	return ok, err
}

func GenerateKey() (pub, priv string, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		return "", "", fmt.Errorf("error generating private key: %v", err)
	}

	{
		derBytes, err := x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			return "", "", fmt.Errorf("error marshaling private key: %v", err)
		}
		pemBlock := &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: derBytes,
		}
		priv = string(pem.EncodeToMemory(pemBlock))
	}
	{
		derBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
		if err != nil {
			return "", "", fmt.Errorf("error marshaling public key: %v", err)
		}
		pemBlock := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: derBytes,
		}
		pub = string(pem.EncodeToMemory(pemBlock))
	}

	return pub, priv, nil
}

// SignJWT will bundle the provided claims into a signed JWT. The provided key
// is assumed to be ECDSA.
//
// If no private key is provided, it will generate a private key. These can
// be retrieved via the SigningKeys() method.
func SignJWT(privKey string, claims jwt.Claims, privateClaims interface{}) (string, error) {
	if privKey == "" {
		var err error
		_, privKey, err = GenerateKey()
		if err != nil {
			return "", err
		}
	}
	var key *ecdsa.PrivateKey
	block, _ := pem.Decode([]byte(privKey))
	if block != nil {
		var err error
		key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
	}

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", err
	}

	raw, err := jwt.Signed(sig).
		Claims(claims).
		Claims(privateClaims).
		CompactSerialize()
	if err != nil {
		return "", err
	}

	return raw, nil
}

// newJWKS converts a pem-encoded public key into JWKS data suitable for a
// verification endpoint response
func NewJWKS(pubKey string) (*jose.JSONWebKeySet, error) {
	block, _ := pem.Decode([]byte(pubKey))
	if block == nil {
		return nil, fmt.Errorf("unable to decode public key")
	}
	input := block.Bytes

	pub, err := x509.ParsePKIXPublicKey(input)
	if err != nil {
		return nil, err
	}
	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key: pub,
			},
		},
	}, nil
}
