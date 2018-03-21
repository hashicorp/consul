package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
)

func TestConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
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

	state := consul.TestServerState(a.Agent.delegate.(*consul.Server))
	t.Log(state.CARoots(nil))

	req, _ := http.NewRequest("GET", "/v1/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.ConnectCARoots(resp, req)
	assert.Nil(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, "")
	assert.Len(value.Roots, 0)
}
