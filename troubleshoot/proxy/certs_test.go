package troubleshoot

import (
	"testing"
	"time"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestValidateCerts(t *testing.T) {

	t.Parallel()

	anHourAgo := timestamppb.New(time.Now().Add(-1 * time.Hour))

	cases := map[string]struct {
		certs         *envoy_admin_v3.Certificates
		expectedError string
	}{
		"cert is nil": {
			certs:         nil,
			expectedError: "certificate object is nil in the proxy configuration",
		},
		"no certificates": {
			certs: &envoy_admin_v3.Certificates{
				Certificates: []*envoy_admin_v3.Certificate{},
			},
			expectedError: "no certificates found",
		},
		"ca expired": {
			certs: &envoy_admin_v3.Certificates{
				Certificates: []*envoy_admin_v3.Certificate{
					{
						CaCert: []*envoy_admin_v3.CertificateDetails{
							{
								ExpirationTime: anHourAgo,
							},
						},
					},
				},
			},
			expectedError: "ca certificate is expired",
		},
		"cert expired": {
			certs: &envoy_admin_v3.Certificates{
				Certificates: []*envoy_admin_v3.Certificate{
					{
						CertChain: []*envoy_admin_v3.CertificateDetails{
							{
								ExpirationTime: anHourAgo,
							},
						},
					},
				},
			},
			expectedError: "certificate chain is expired",
		},
	}

	ts := Troubleshoot{}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			messages := ts.validateCerts(tc.certs)

			var outputErrors string
			for _, msgError := range messages.Errors() {
				outputErrors += msgError.Message
				outputErrors += msgError.PossibleActions
			}
			if tc.expectedError == "" {
				require.True(t, messages.Success())
			} else {
				require.Contains(t, outputErrors, tc.expectedError)
			}

		})
	}

}
