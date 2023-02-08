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

	x := []struct {
		certs         *envoy_admin_v3.Certificates
		expectedError string
	}{
		{
			certs:         nil,
			expectedError: "certs object is nil",
		},
		{
			certs: &envoy_admin_v3.Certificates{
				Certificates: []*envoy_admin_v3.Certificate{},
			},
			expectedError: "no certificates provided",
		},
		{
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
			expectedError: "Ca cert is expired",
		},
		{
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
			expectedError: "cert chain is expired",
		},
	}

	ts := Troubleshoot{}
	for _, tc := range x {
		err := ts.validateCerts(tc.certs)
		if tc.expectedError != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedError)
		}
	}

}
