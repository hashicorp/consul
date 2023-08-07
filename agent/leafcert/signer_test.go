// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leafcert

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

// testSigner implements NetRPC and handles leaf signing operations
type testSigner struct {
	caLock    sync.Mutex
	ca        *structs.CARoot
	prevRoots []*structs.CARoot // remember prior ones

	IDGenerator *atomic.Uint64
	RootsReader *testRootsReader

	signCallLock       sync.Mutex
	signCallErrors     []error
	signCallErrorCount uint64
	signCallCapture    []*structs.CASignRequest
}

var _ CertSigner = (*testSigner)(nil)

var ReplyWithExpiredCert = errors.New("reply with expired cert")

func newTestSigner(t *testing.T, idGenerator *atomic.Uint64, rootsReader *testRootsReader) *testSigner {
	if idGenerator == nil {
		idGenerator = &atomic.Uint64{}
	}
	if rootsReader == nil {
		rootsReader = newTestRootsReader(t)
	}
	s := &testSigner{
		IDGenerator: idGenerator,
		RootsReader: rootsReader,
	}
	return s
}

func (s *testSigner) SetSignCallErrors(errs ...error) {
	s.signCallLock.Lock()
	defer s.signCallLock.Unlock()
	s.signCallErrors = append(s.signCallErrors, errs...)
}

func (s *testSigner) GetSignCallErrorCount() uint64 {
	s.signCallLock.Lock()
	defer s.signCallLock.Unlock()
	return s.signCallErrorCount
}

func (s *testSigner) UpdateCA(t *testing.T, ca *structs.CARoot) *structs.CARoot {
	if ca == nil {
		ca = connect.TestCA(t, nil)
	}
	roots := &structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  connect.TestTrustDomain,
		Roots:        []*structs.CARoot{ca},
		QueryMeta:    structs.QueryMeta{Index: s.nextIndex()},
	}

	// Update the signer first.
	s.caLock.Lock()
	{
		s.ca = ca
		roots.Roots = append(roots.Roots, s.prevRoots...)
		// Remember for the next rotation.
		dup := ca.Clone()
		dup.Active = false
		s.prevRoots = append(s.prevRoots, dup)
	}
	s.caLock.Unlock()

	// Then trigger an event when updating the roots.
	s.RootsReader.Set(roots)

	return ca
}

func (s *testSigner) nextIndex() uint64 {
	return s.IDGenerator.Add(1)
}

func (s *testSigner) getCA() *structs.CARoot {
	s.caLock.Lock()
	defer s.caLock.Unlock()
	return s.ca
}

func (s *testSigner) GetCapture(idx int) *structs.CASignRequest {
	s.signCallLock.Lock()
	defer s.signCallLock.Unlock()
	if len(s.signCallCapture) > idx {
		return s.signCallCapture[idx]
	}

	return nil
}

func (s *testSigner) SignCert(ctx context.Context, req *structs.CASignRequest) (*structs.IssuedCert, error) {
	useExpiredCert := false
	s.signCallLock.Lock()
	s.signCallCapture = append(s.signCallCapture, req)
	if len(s.signCallErrors) > 0 {
		err := s.signCallErrors[0]
		s.signCallErrors = s.signCallErrors[1:]
		if err == ReplyWithExpiredCert {
			useExpiredCert = true
		} else if err != nil {
			s.signCallErrorCount++
			s.signCallLock.Unlock()
			return nil, err
		}
	}
	s.signCallLock.Unlock()

	// parts of this were inlined from CAManager and the connect ca provider
	ca := s.getCA()
	if ca == nil {
		return nil, fmt.Errorf("must call UpdateCA at least once")
	}

	csr, err := connect.ParseCSR(req.CSR)
	if err != nil {
		return nil, fmt.Errorf("error parsing CSR: %w", err)
	}

	connect.HackSANExtensionForCSR(csr)

	spiffeID, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing CSR URI: %w", err)
	}

	serviceID, isService := spiffeID.(*connect.SpiffeIDService)
	if !isService {
		return nil, fmt.Errorf("unexpected spiffeID type %T", spiffeID)
	}

	signer, err := connect.ParseSigner(ca.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("error parsing CA signing key: %w", err)
	}

	keyId, err := connect.KeyId(signer.Public())
	if err != nil {
		return nil, fmt.Errorf("error forming CA key id from public key: %w", err)
	}

	subjectKeyID, err := connect.KeyId(csr.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("error forming subject key id from public key: %w", err)
	}

	caCert, err := connect.ParseCert(ca.RootCert)
	if err != nil {
		return nil, fmt.Errorf("error parsing CA root cert pem: %w", err)
	}

	const expiration = 10 * time.Minute

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: big.NewInt(int64(s.nextIndex())),
		URIs:         csr.URIs,
		Signature:    csr.Signature,
		// We use the correct signature algorithm for the CA key we are signing with
		// regardless of the algorithm used to sign the CSR signature above since
		// the leaf might use a different key type.
		SignatureAlgorithm:    connect.SigAlgoForKey(signer),
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
		PublicKey:             csr.PublicKey,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDataEncipherment |
			x509.KeyUsageKeyAgreement |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		NotAfter:       now.Add(expiration),
		NotBefore:      now,
		AuthorityKeyId: keyId,
		SubjectKeyId:   subjectKeyID,
		DNSNames:       csr.DNSNames,
		IPAddresses:    csr.IPAddresses,
	}

	if useExpiredCert {
		template.NotBefore = time.Now().Add(-13 * time.Hour)
		template.NotAfter = time.Now().Add(-1 * time.Hour)
	}

	// Create the certificate, PEM encode it and return that value.
	var buf bytes.Buffer
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, caCert, csr.PublicKey, signer)
	if err != nil {
		return nil, fmt.Errorf("error creating cert pem from CSR: %w", err)
	}

	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return nil, fmt.Errorf("error encoding cert pem into text: %w", err)
	}

	leafPEM := buf.String()

	leafCert, err := connect.ParseCert(leafPEM)
	if err != nil {
		return nil, fmt.Errorf("error parsing cert from generated leaf pem: %w", err)
	}

	index := s.nextIndex()
	return &structs.IssuedCert{
		SerialNumber: connect.EncodeSerialNumber(leafCert.SerialNumber),
		CertPEM:      leafPEM,
		Service:      serviceID.Service,
		ServiceURI:   leafCert.URIs[0].String(),
		ValidAfter:   leafCert.NotBefore,
		ValidBefore:  leafCert.NotAfter,
		RaftIndex: structs.RaftIndex{
			CreateIndex: index,
			ModifyIndex: index,
		},
	}, nil
}
