package consul

import (
	"github.com/stretchr/testify/require"
	"log"
	"net"
	"os"
	"testing"
)

func TestAutoEncrypt_resolveAddr(t *testing.T) {
	type args struct {
		rawHost string
		logger  *log.Logger
	}
	tests := []struct {
		name    string
		args    args
		ips     []net.IP
		wantErr bool
	}{
		{
			name: "host without port",
			args: args{
				"127.0.0.1",
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			wantErr: false,
		},
		{
			name: "host with port",
			args: args{
				"127.0.0.1:1234",
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			wantErr: false,
		},
		{
			name: "host with broken port",
			args: args{
				"127.0.0.1:xyz",
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			wantErr: false,
		},
		{
			name: "not an address",
			args: args{
				"abc",
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolveAddr(tt.args.rawHost, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveAddr error: %v, wantErr: %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.ips, ips)
		})
	}
}

func TestAutoEncrypt_missingPortError(t *testing.T) {
	host := "127.0.0.1"
	_, _, err := net.SplitHostPort(host)
	require.True(t, missingPortError(host, err))

	host = "127.0.0.1:1234"
	_, _, err = net.SplitHostPort(host)
	require.False(t, missingPortError(host, err))
}
