package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
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
	assert.Nil(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, ca2.ID)
	assert.Len(value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}
}
