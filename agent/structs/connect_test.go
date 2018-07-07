package structs

import (
	"reflect"
	"testing"
)

func TestConnectManagedProxy_ParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		want    *ConnectManagedProxyConfig
		wantErr bool
	}{
		{
			name:    "empty",
			config:  nil,
			want:    &ConnectManagedProxyConfig{},
			wantErr: false,
		},
		{
			name: "specified",
			config: map[string]interface{}{
				"bind_address": "127.0.0.1",
				"bind_port":    1234,
			},
			want: &ConnectManagedProxyConfig{
				BindAddress: "127.0.0.1",
				BindPort:    1234,
			},
			wantErr: false,
		},
		{
			name: "stringy port",
			config: map[string]interface{}{
				"bind_address": "127.0.0.1",
				"bind_port":    "1234",
			},
			want: &ConnectManagedProxyConfig{
				BindAddress: "127.0.0.1",
				BindPort:    1234,
			},
			wantErr: false,
		},
		{
			name: "empty addr",
			config: map[string]interface{}{
				"bind_address": "",
				"bind_port":    "1234",
			},
			want: &ConnectManagedProxyConfig{
				BindAddress: "",
				BindPort:    1234,
			},
			wantErr: false,
		},
		{
			name: "empty port",
			config: map[string]interface{}{
				"bind_address": "127.0.0.1",
				"bind_port":    "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "junk address",
			config: map[string]interface{}{
				"bind_address": 42,
				"bind_port":    "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "zero port, missing addr",
			config: map[string]interface{}{
				"bind_port": 0,
			},
			want: &ConnectManagedProxyConfig{
				BindPort: 0,
			},
			wantErr: false,
		},
		{
			name: "extra fields present",
			config: map[string]interface{}{
				"bind_port": 1234,
				"flamingos": true,
				"upstream": []map[string]interface{}{
					{"foo": "bar"},
				},
			},
			want: &ConnectManagedProxyConfig{
				BindPort: 1234,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ConnectManagedProxy{
				Config: tt.config,
			}
			got, err := p.ParseConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectManagedProxy.ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConnectManagedProxy.ParseConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
