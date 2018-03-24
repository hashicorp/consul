package consul

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/go-testing-interface"
	"github.com/mitchellh/mapstructure"
)

// ConnectCA manages the Connect CA.
type ConnectCA struct {
	// srv is a pointer back to the server.
	srv *Server
}

// ConfigurationSet updates the configuration for the CA.
//
// NOTE(mitchellh): This whole implementation is temporary until the real
// CA plugin work comes in. For now, this is only used to configure a single
// static CA root.
func (s *ConnectCA) ConfigurationSet(
	args *structs.CAConfiguration,
	reply *interface{}) error {
	// NOTE(mitchellh): This is the temporary hardcoding of a static CA
	// provider. This will allow us to test agent implementations and so on
	// with an incomplete CA for now.
	if args.Provider != "static" {
		return fmt.Errorf("The CA provider can only be 'static' for now")
	}

	// Config is the configuration allowed for our static provider
	var config struct {
		Name          string
		CertPEM       string
		PrivateKeyPEM string
		Generate      bool
	}
	if err := mapstructure.Decode(args.Config, &config); err != nil {
		return fmt.Errorf("error decoding config: %s", err)
	}

	// Basic validation so demos aren't super jank
	if config.Name == "" {
		return fmt.Errorf("Name must be set")
	}
	if config.CertPEM == "" || config.PrivateKeyPEM == "" {
		if !config.Generate {
			return fmt.Errorf(
				"CertPEM and PrivateKeyPEM must be set, or Generate must be true")
		}
	}

	// Convenience to auto-generate the cert
	if config.Generate {
		ca := connect.TestCA(&testing.RuntimeT{}, nil)
		config.CertPEM = ca.RootCert
		config.PrivateKeyPEM = ca.SigningKey
	}

	// TODO(mitchellh): verify that the private key is valid for the cert

	// Generate an ID for this
	id, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}

	// Get the highest index
	state := s.srv.fsm.State()
	idx, _, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	// Commit
	resp, err := s.srv.raftApply(structs.ConnectCARequestType, &structs.CARequest{
		Op:    structs.CAOpSet,
		Index: idx,
		Roots: []*structs.CARoot{
			&structs.CARoot{
				ID:         id,
				Name:       config.Name,
				RootCert:   config.CertPEM,
				SigningKey: config.PrivateKeyPEM,
				Active:     true,
			},
		},
	})
	if err != nil {
		s.srv.logger.Printf("[ERR] consul.test: Apply failed %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

// Roots returns the currently trusted root certificates.
func (s *ConnectCA) Roots(
	args *structs.DCSpecificRequest,
	reply *structs.IndexedCARoots) error {
	// Forward if necessary
	if done, err := s.srv.forward("ConnectCA.Roots", args, args, reply); done {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, roots, err := state.CARoots(ws)
			if err != nil {
				return err
			}

			reply.Index, reply.Roots = index, roots
			if reply.Roots == nil {
				reply.Roots = make(structs.CARoots, 0)
			}

			// The API response must NEVER contain the secret information
			// such as keys and so on. We use a whitelist below to copy the
			// specific fields we want to expose.
			for i, r := range reply.Roots {
				// IMPORTANT: r must NEVER be modified, since it is a pointer
				// directly to the structure in the memdb store.

				reply.Roots[i] = &structs.CARoot{
					ID:        r.ID,
					Name:      r.Name,
					RootCert:  r.RootCert,
					RaftIndex: r.RaftIndex,
					Active:    r.Active,
				}

				if r.Active {
					reply.ActiveRootID = r.ID
				}
			}

			return nil
		},
	)
}

// Sign signs a certificate for a service.
//
// NOTE(mitchellh): There is a LOT missing from this. I do next to zero
// validation of the incoming CSR, the way the cert is signed probably
// isn't right, we're not using enough of the CSR fields, etc.
func (s *ConnectCA) Sign(
	args *structs.CASignRequest,
	reply *structs.IssuedCert) error {
	// Parse the CSR
	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	// Parse the SPIFFE ID
	spiffeId, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return err
	}
	serviceId, ok := spiffeId.(*connect.SpiffeIDService)
	if !ok {
		return fmt.Errorf("SPIFFE ID in CSR must be a service ID")
	}

	// Get the currently active root
	state := s.srv.fsm.State()
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("no active CA found")
	}

	// Determine the signing certificate. It is the set signing cert
	// unless that is empty, in which case it is identically to the public
	// cert.
	certPem := root.SigningCert
	if certPem == "" {
		certPem = root.RootCert
	}

	// Parse the CA cert and signing key from the root
	caCert, err := connect.ParseCert(certPem)
	if err != nil {
		return fmt.Errorf("error parsing CA cert: %s", err)
	}
	signer, err := connect.ParseSigner(root.SigningKey)
	if err != nil {
		return fmt.Errorf("error parsing signing key: %s", err)
	}

	// The serial number for the cert. NOTE(mitchellh): in the final
	// implementation this should be monotonically increasing based on
	// some raft state.
	sn, err := rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(159), nil))
	if err != nil {
		return fmt.Errorf("error generating serial number: %s", err)
	}

	// Create the keyId for the cert from the signing public key.
	keyId, err := connect.KeyId(signer.Public())
	if err != nil {
		return err
	}

	// Cert template for generation
	template := x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: serviceId.Service},
		URIs:                  csr.URIs,
		Signature:             csr.Signature,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
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
		NotAfter:       time.Now().Add(3 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: keyId,
		SubjectKeyId:   keyId,
	}

	// Create the certificate, PEM encode it and return that value.
	var buf bytes.Buffer
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, caCert, signer.Public(), signer)
	if err != nil {
		return fmt.Errorf("error generating certificate: %s", err)
	}
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return fmt.Errorf("error encoding private key: %s", err)
	}

	// Set the response
	*reply = structs.IssuedCert{
		SerialNumber: connect.HexString(template.SerialNumber.Bytes()),
		CertPEM:      buf.String(),
		Service:      serviceId.Service,
		ServiceURI:   template.URIs[0].String(),
		ValidAfter:   template.NotBefore,
		ValidBefore:  template.NotAfter,
	}

	return nil
}
