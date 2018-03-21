package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// You can verify a given leaf with a given root using:
//
//   $ openssl verify -verbose -CAfile ca2-ca-vault.cert.pem ca1-svc-db.cert.pem
//
// Note that to verify via the cross-signed intermediate, openssl requires it to
// be bundled with the _root_ CA bundle and will ignore the cert if it's passed
// with the subject. You can do that with:
//
//   $ openssl verify -verbose -CAfile \
//      <(cat ca1-ca-consul-internal.cert.pem ca2-xc-by-ca1.cert.pem) \
//      ca2-svc-db.cert.pem
//   ca2-svc-db.cert.pem: OK
//
// Note that the same leaf and root without the intermediate should fail:
//
//  $ openssl verify -verbose -CAfile ca1-ca-consul-internal.cert.pem ca2-svc-db.cert.pem
//  ca2-svc-db.cert.pem: CN = db
//  error 20 at 0 depth lookup:unable to get local issuer certificate
//
// NOTE: THIS IS A QUIRK OF OPENSSL; in Connect we will distribute the roots
// alone and stable intermediates like the XC cert to the _leaf_.

var clusterID = "11111111-2222-3333-4444-555555555555"
var cAs = []string{"Consul Internal", "Vault"}
var services = []string{"web", "db", "cache"}
var slugRe = regexp.MustCompile("[^a-zA-Z0-9]+")
var serial int64

type caInfo struct {
	id   int
	name string
	slug string
	uri  *url.URL
	pk   *ecdsa.PrivateKey
	cert *x509.Certificate
}

func main() {
	// Make CA certs
	caInfos := make(map[string]caInfo)
	var previousCA *caInfo
	for idx, name := range cAs {
		ca := caInfo{
			id:   idx + 1,
			name: name,
			slug: strings.ToLower(slugRe.ReplaceAllString(name, "-")),
		}
		pk, err := makePK(fmt.Sprintf("ca%d-ca-%s.key.pem", ca.id, ca.slug))
		if err != nil {
			log.Fatal(err)
		}
		ca.pk = pk
		caURI, err := url.Parse(fmt.Sprintf("spiffe://%s.consul", clusterID))
		if err != nil {
			log.Fatal(err)
		}
		ca.uri = caURI
		cert, err := makeCACert(ca, previousCA)
		if err != nil {
			log.Fatal(err)
		}
		ca.cert = cert
		caInfos[name] = ca
		previousCA = &ca
	}

	// For each CA, make a leaf cert for each service
	for _, ca := range caInfos {
		for _, svc := range services {
			_, err := makeLeafCert(ca, svc)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func makePK(path string) (*ecdsa.PrivateKey, error) {
	log.Printf("Writing PK file: %s", path)
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	bs, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	err = writePEM(path, "EC PRIVATE KEY", bs)
	return priv, nil
}

func makeCACert(ca caInfo, previousCA *caInfo) (*x509.Certificate, error) {
	path := fmt.Sprintf("ca%d-ca-%s.cert.pem", ca.id, ca.slug)
	log.Printf("Writing CA cert file: %s", path)
	serial++
	subj := pkix.Name{
		CommonName: ca.name,
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      subj,
		// New in go 1.10
		URIs: []*url.URL{ca.uri},
		// Add DNS name constraint
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{ca.uri.Hostname()},
		SignatureAlgorithm:          x509.ECDSAWithSHA256,
		BasicConstraintsValid:       true,
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		IsCA:                        true,
		NotAfter:                    time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:                   time.Now(),
		AuthorityKeyId:              keyID(&ca.pk.PublicKey),
		SubjectKeyId:                keyID(&ca.pk.PublicKey),
	}
	bs, err := x509.CreateCertificate(rand.Reader, &template, &template,
		&ca.pk.PublicKey, ca.pk)
	if err != nil {
		return nil, err
	}

	err = writePEM(path, "CERTIFICATE", bs)
	if err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(bs)
	if err != nil {
		return nil, err
	}

	if previousCA != nil {
		// Also create cross-signed cert as we would use during rotation between
		// previous CA and this one.
		template.AuthorityKeyId = keyID(&previousCA.pk.PublicKey)
		bs, err := x509.CreateCertificate(rand.Reader, &template,
			previousCA.cert, &ca.pk.PublicKey, previousCA.pk)
		if err != nil {
			return nil, err
		}

		path := fmt.Sprintf("ca%d-xc-by-ca%d.cert.pem", ca.id, previousCA.id)
		err = writePEM(path, "CERTIFICATE", bs)
		if err != nil {
			return nil, err
		}
	}

	return cert, err
}

func keyID(pub *ecdsa.PublicKey) []byte {
	// This is not standard; RFC allows any unique identifier as long as they
	// match in subject/authority chains but suggests specific hashing of DER
	// bytes of public key including DER tags. I can't be bothered to do esp.
	// since ECDSA keys don't have a handy way to marshal the publick key alone.
	h := sha256.New()
	h.Write(pub.X.Bytes())
	h.Write(pub.Y.Bytes())
	return h.Sum([]byte{})
}

func makeLeafCert(ca caInfo, svc string) (*x509.Certificate, error) {
	svcURI := ca.uri
	svcURI.Path = "/ns/default/dc/dc01/svc/" + svc

	keyPath := fmt.Sprintf("ca%d-svc-%s.key.pem", ca.id, svc)
	cPath := fmt.Sprintf("ca%d-svc-%s.cert.pem", ca.id, svc)

	pk, err := makePK(keyPath)
	if err != nil {
		return nil, err
	}

	log.Printf("Writing Service Cert: %s", cPath)

	serial++
	subj := pkix.Name{
		CommonName: svc,
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      subj,
		// New in go 1.10
		URIs:                  []*url.URL{svcURI},
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDataEncipherment |
			x509.KeyUsageKeyAgreement | x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		NotAfter:       time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: keyID(&ca.pk.PublicKey),
		SubjectKeyId:   keyID(&pk.PublicKey),
	}
	bs, err := x509.CreateCertificate(rand.Reader, &template, ca.cert,
		&pk.PublicKey, ca.pk)
	if err != nil {
		return nil, err
	}

	err = writePEM(cPath, "CERTIFICATE", bs)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(bs)
}

func writePEM(name, typ string, bs []byte) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: typ, Bytes: bs})
}
