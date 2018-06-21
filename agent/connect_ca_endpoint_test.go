package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	ca "github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
)

func TestConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "connect { enabled = false }")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ConnectCARoots(resp, req)
	assert.Nil(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, "")
	assert.Len(value.Roots, 0)
}

func TestConnectCARoots_list(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Set some CAs. Note that NewTestAgent already bootstraps one CA so this just
	// adds a second and makes it active.
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// List
	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ConnectCARoots(resp, req)
	assert.NoError(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, ca2.ID)
	assert.Len(value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}
}

func TestConnectCAConfig(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	root := connect.TestCA(t, nil)
	expected := &structs.ConsulCAProviderConfig{}

	// Get the initial config.
	{
		req, _ := http.NewRequest("GET", "/v1/connect/ca/configuration", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.ConnectCAConfiguration(resp, req)
		assert.NoError(err)

		value := obj.(structs.CAConfiguration)
		parsed, err := ca.ParseConsulCAConfig(value.Config)
		assert.NoError(err)
		assert.Equal("consul", value.Provider)
		assert.Equal(expected, parsed)
	}

	// Set the config.
	{
		conf := fmt.Sprintf(`
			{
				"Provider": "consul",
				"Config": {
					"PrivateKey": "%s",
					"RootCert": "%s"
				}
			}`,
			strings.Replace(root.SigningKey, "\n", "\\n", -1),
			strings.Replace(root.RootCert, "\n", "\\n", -1))
		body := bytes.NewBuffer([]byte(conf))
		req, _ := http.NewRequest("PUT", "/v1/connect/ca/configuration", body)
		resp := httptest.NewRecorder()
		_, err := a.srv.ConnectCAConfiguration(resp, req)
		assert.NoError(err)
	}

	// The config should be updated now.
	{
		expected.PrivateKey = "hidden"
		expected.RootCert = root.RootCert
		req, _ := http.NewRequest("GET", "/v1/connect/ca/configuration", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.ConnectCAConfiguration(resp, req)
		assert.NoError(err)

		value := obj.(structs.CAConfiguration)
		parsed, err := ca.ParseConsulCAConfig(value.Config)
		assert.NoError(err)
		assert.Equal("consul", value.Provider)
		assert.Equal(expected, parsed)
	}
}
