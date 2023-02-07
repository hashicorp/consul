package troubleshoot

import (
	"fmt"
	"time"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/encoding/protojson"
)

func (t *Troubleshoot) validateCerts(certs *envoy_admin_v3.Certificates) error {

	// TODO: we can probably warn if the expiration date is close
	var resultErr error
	now := time.Now()

	for _, cert := range certs.GetCertificates() {
		for _, cacert := range cert.GetCaCert() {
			if now.After(cacert.GetExpirationTime().AsTime()) {
				resultErr = multierror.Append(resultErr, fmt.Errorf("Ca cert is expired"))
			}

		}
		for _, cc := range cert.GetCertChain() {
			if now.After(cc.GetExpirationTime().AsTime()) {
				resultErr = multierror.Append(resultErr, fmt.Errorf("cert chain is expired"))
			}
		}
	}
	return resultErr
}

func (t *Troubleshoot) getEnvoyCerts() (*envoy_admin_v3.Certificates, error) {

	certsRaw, err := t.request("certs?format=json")
	if err != nil {
		return nil, fmt.Errorf("error in requesting Envoy Admin API /certs endpoint: %w", err)
	}

	certs := &envoy_admin_v3.Certificates{}

	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = unmarshal.Unmarshal(certsRaw, certs)
	if err != nil {
		return nil, fmt.Errorf("error in unmarshalling /certs response: %w", err)
	}

	t.envoyCerts = certs
	return certs, nil
}
