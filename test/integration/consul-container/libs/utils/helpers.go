// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/consul/api"
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

// Generates a private and public key pair that is for signing
// JWT.
func GenerateKey() (pub, priv string, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		return "", "", fmt.Errorf("error generating private key: %w", err)
	}

	{
		derBytes, err := x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			return "", "", fmt.Errorf("error marshaling private key: %w", err)
		}
		priv = string(pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: derBytes,
		}))
	}
	{
		derBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
		if err != nil {
			return "", "", fmt.Errorf("error marshaling public key: %w", err)
		}
		pub = string(pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: derBytes,
		}))
	}

	return pub, priv, nil
}

// SignJWT will bundle the provided claims into a signed JWT. The provided key
// is assumed to be ECDSA.
//
// If no private key is provided, it will generate a private key. These can
// be retrieved via the SigningKeys() method.
func SignJWT(privKey string, claims jwt.Claims, privateClaims interface{}) (string, error) {
	var err error
	if privKey == "" {
		_, privKey, err = GenerateKey()
		if err != nil {
			return "", err
		}
	}
	var key *ecdsa.PrivateKey
	block, _ := pem.Decode([]byte(privKey))
	if block != nil {
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
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("unable to decode public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
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
