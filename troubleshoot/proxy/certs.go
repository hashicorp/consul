// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package troubleshoot

import (
	"fmt"
	"time"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/consul/troubleshoot/validate"
)

func (t *Troubleshoot) validateCerts(certs *envoy_admin_v3.Certificates) validate.Messages {

	var certMessages validate.Messages
	// TODO: we can probably warn if the expiration date is close
	now := time.Now()

	if certs == nil {
		msg := validate.Message{
			Success: false,
			Message: "Certificate object is nil in the proxy configuration",
			PossibleActions: []string{
				"Check the logs of the Consul agent configuring the local proxy and ensure XDS updates are being sent to the proxy",
			},
		}
		return []validate.Message{msg}
	}

	if len(certs.GetCertificates()) == 0 {
		msg := validate.Message{
			Success: false,
			Message: "No certificates found",
			PossibleActions: []string{
				"Check the logs of the Consul agent configuring the local proxy and ensure XDS updates are being sent to the proxy",
			},
		}
		return []validate.Message{msg}
	}

	for _, cert := range certs.GetCertificates() {
		for _, cacert := range cert.GetCaCert() {
			if now.After(cacert.GetExpirationTime().AsTime()) {
				msg := validate.Message{
					Success: false,
					Message: "CA certificate is expired",
					PossibleActions: []string{
						"Check the logs of the Consul agent configuring the local proxy and ensure XDS updates are being sent to the proxy",
					},
				}
				certMessages = append(certMessages, msg)
			}

		}
		for _, cc := range cert.GetCertChain() {
			if now.After(cc.GetExpirationTime().AsTime()) {
				msg := validate.Message{
					Success: false,
					Message: "Certificate chain is expired",
					PossibleActions: []string{
						"Check the logs of the Consul agent configuring the local proxy and ensure XDS updates are being sent to the proxy",
					},
				}
				certMessages = append(certMessages, msg)
			}
		}
	}
	return certMessages
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
