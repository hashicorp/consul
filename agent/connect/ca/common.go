// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"bytes"
	"crypto/x509"
	"fmt"

	"github.com/hashicorp/consul/agent/connect"
)

func validateSetIntermediate(intermediatePEM, rootPEM string, spiffeID *connect.SpiffeIDSigning) error {
	// Get the key from the incoming intermediate cert so we can compare it
	// to the currently stored key.
	intermediate, err := connect.ParseCert(intermediatePEM)
	if err != nil {
		return fmt.Errorf("error parsing intermediate PEM: %v", err)
	}

	// Validate the remaining fields and make sure the intermediate validates against
	// the given root cert.
	if !intermediate.IsCA {
		return fmt.Errorf("intermediate is not a CA certificate")
	}
	if uriCount := len(intermediate.URIs); uriCount != 1 {
		return fmt.Errorf("incoming intermediate cert has unexpected number of URIs: %d", uriCount)
	}
	if got, want := intermediate.URIs[0].String(), spiffeID.URI().String(); got != want {
		return fmt.Errorf("incoming cert URI %q does not match current URI: %q", got, want)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(rootPEM))
	_, err = intermediate.Verify(x509.VerifyOptions{
		Roots: pool,
	})
	if err != nil {
		return fmt.Errorf("could not verify intermediate cert against root: %v", err)
	}

	return nil
}

func validateIntermediateSignedByPrivateKey(intermediatePEM string, privateKey string) error {
	intermediate, err := connect.ParseCert(intermediatePEM)
	if err != nil {
		return fmt.Errorf("error parsing intermediate PEM: %v", err)
	}

	privKey, err := connect.ParseSigner(privateKey)
	if err != nil {
		return err
	}

	// Compare the two keys to make sure they match.
	b1, err := x509.MarshalPKIXPublicKey(intermediate.PublicKey)
	if err != nil {
		return err
	}
	b2, err := x509.MarshalPKIXPublicKey(privKey.Public())
	if err != nil {
		return err
	}
	if !bytes.Equal(b1, b2) {
		return fmt.Errorf("intermediate cert is for a different private key")
	}
	return nil
}

func validateSignIntermediate(csr *x509.CertificateRequest, spiffeID *connect.SpiffeIDSigning) error {
	// We explicitly _don't_ require that the CSR has a valid SPIFFE signing URI
	// SAN because AWS PCA doesn't let us set one :(. We need to relax it here
	// otherwise it would be impossible to migrate from built-in provider to AWS
	// in multiple DCs without downtime. Nothing in Connect actually checks that
	// currently so this is OK for now but it's sad we have to break the SPIFFE
	// spec for AWS sake. Hopefully they'll add that ability soon.
	uriCount := len(csr.URIs)
	if uriCount > 0 {
		if uriCount != 1 {
			return fmt.Errorf("incoming CSR has unexpected number of URIs: %d", uriCount)
		}
		certURI, err := connect.ParseCertURI(csr.URIs[0])
		if err != nil {
			return err
		}

		// Verify that the trust domain is valid.
		if !spiffeID.CanSign(certURI) {
			return fmt.Errorf("incoming CSR domain %q is not valid for our domain %q",
				certURI.URI().String(), spiffeID.URI().String())
		}
	}
	return nil
}
