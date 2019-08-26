package consul

import (
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestAutoEncrypt_RequestAutoEncryptCerts(t *testing.T) {
	dir1, c1 := testClient(t)
	defer os.RemoveAll(dir1)
	defer c1.Shutdown()
	servers := []string{"localhost"}
	port := 8301
	token := ""
	interruptCh := make(chan struct{})
	doneCh := make(chan struct{})
	var err error
	go func() {
		_, _, err = c1.RequestAutoEncryptCerts(servers, port, token, interruptCh)
		close(doneCh)
	}()
	select {
	case <-doneCh:
		// since there are no servers at this port, we shouldn't be
		// done and this should be an error of some sorts that happened
		// in the setup phase before entering the for loop in
		// RequestAutoEncryptCerts.
		require.NoError(t, err)
	case <-time.After(50 * time.Millisecond):
		// this is the happy case since auto encrypt is in its loop to
		// try to request certs.
		interruptCh <- struct{}{}
	}
}
