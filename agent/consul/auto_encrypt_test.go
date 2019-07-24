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
		rawHost     string
		defaultPort int
		logger      *log.Logger
	}
	tests := []struct {
		name    string
		args    args
		ips     []net.IP
		port    int
		wantErr bool
	}{
		{
			name: "host without port",
			args: args{
				"127.0.0.1",
				8300,
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			port:    8300,
			wantErr: false,
		},
		{
			name: "host with port",
			args: args{
				"127.0.0.1:1234",
				8300,
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			port:    1234,
			wantErr: false,
		},
		{
			name: "host with broken port",
			args: args{
				"127.0.0.1:xyz",
				8300,
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			port:    8300,
			wantErr: false,
		},
		{
			name: "not an address",
			args: args{
				"abc",
				8300,
				log.New(os.Stderr, "", log.LstdFlags),
			},
			ips:     nil,
			port:    8300,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, port, err := resolveAddr(tt.args.rawHost, tt.args.defaultPort, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveAddr error: %v, wantErr: %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.ips, ips)
			require.Equal(t, tt.port, port)
		})
	}
}
