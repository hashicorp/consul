package peering

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestValidatePeeringToken(t *testing.T) {
	type testCase struct {
		name    string
		token   *structs.PeeringToken
		wantErr error
	}

	tt := []testCase{
		{
			name:    "empty",
			token:   &structs.PeeringToken{},
			wantErr: errPeeringTokenEmptyServerAddresses,
		},
		{
			name: "empty CA",
			token: &structs.PeeringToken{
				CA: []string{},
			},
			wantErr: errPeeringTokenEmptyServerAddresses,
		},
		{
			name: "invalid CA",
			token: &structs.PeeringToken{
				CA: []string{"notavalidcert"},
			},
			wantErr: errors.New("peering token invalid CA: no PEM-encoded data found"),
		},
		{
			name: "invalid CA cert",
			token: &structs.PeeringToken{
				CA: []string{invalidCA},
			},
			wantErr: errors.New("peering token invalid CA: x509: malformed certificate"),
		},
		{
			name: "invalid address port",
			token: &structs.PeeringToken{
				CA:              []string{validCA},
				ServerAddresses: []string{"1.2.3.4"},
			},
			wantErr: &errPeeringInvalidServerAddress{
				"1.2.3.4",
			},
		},
		{
			name: "invalid server name",
			token: &structs.PeeringToken{
				CA:              []string{validCA},
				ServerAddresses: []string{"1.2.3.4:80"},
			},
			wantErr: errPeeringTokenEmptyServerName,
		},
		{
			name: "invalid peer ID",
			token: &structs.PeeringToken{
				CA:              []string{validCA},
				ServerAddresses: []string{validAddress},
				ServerName:      validServerName,
			},
			wantErr: errPeeringTokenEmptyPeerID,
		},
		{
			name: "valid token",
			token: &structs.PeeringToken{
				CA:              []string{validCA},
				ServerAddresses: []string{validAddress},
				ServerName:      validServerName,
				PeerID:          validPeerID,
			},
		},
		{
			name: "valid token with hostname address",
			token: &structs.PeeringToken{
				CA:              []string{validCA},
				ServerAddresses: []string{validHostnameAddress},
				ServerName:      validServerName,
				PeerID:          validPeerID,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePeeringToken(tc.token)
			if tc.wantErr != nil {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				require.Contains(t, err.Error(), tc.wantErr.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
