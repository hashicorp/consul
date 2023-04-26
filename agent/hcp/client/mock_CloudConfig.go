package client

import (
	"crypto/tls"
	"errors"
	"net/url"

	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
	"github.com/hashicorp/hcp-sdk-go/profile"
	"golang.org/x/oauth2"
)

type mockHCPCfg struct{}

func (m *mockHCPCfg) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "test-token",
	}, nil
}

func (m *mockHCPCfg) APITLSConfig() *tls.Config     { return nil }
func (m *mockHCPCfg) SCADAAddress() string          { return "" }
func (m *mockHCPCfg) SCADATLSConfig() *tls.Config   { return &tls.Config{} }
func (m *mockHCPCfg) APIAddress() string            { return "" }
func (m *mockHCPCfg) PortalURL() *url.URL           { return &url.URL{} }
func (m *mockHCPCfg) Profile() *profile.UserProfile { return &profile.UserProfile{} }

type MockCloudCfg struct{}

func (m MockCloudCfg) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	return &mockHCPCfg{}, nil
}

type MockErrCloudCfg struct{}

func (m MockErrCloudCfg) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	return nil, errors.New("test bad HCP config")
}
