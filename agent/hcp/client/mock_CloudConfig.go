package client

import (
	"crypto/tls"
	"net/url"

	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
	"github.com/hashicorp/hcp-sdk-go/profile"
	"github.com/hashicorp/hcp-sdk-go/resource"
	"golang.org/x/oauth2"
)

const testResourceID = "organization/ccbdd191-5dc3-4a73-9e05-6ac30ca67992/project/36019e0d-ed59-4df6-9990-05bb7fc793b6/hashicorp.consul.linked-cluster/prod-on-prem"

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
func (m *mockHCPCfg) Profile() *profile.UserProfile { return nil }

type MockCloudCfg struct {
	ConfigErr   error
	ResourceErr error
}

func (m MockCloudCfg) Resource() (resource.Resource, error) {
	r, _ := resource.FromString(testResourceID)
	return r, m.ResourceErr
}

func (m MockCloudCfg) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	return &mockHCPCfg{}, m.ConfigErr
}
