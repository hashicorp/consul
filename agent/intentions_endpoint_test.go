package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntentionsList_empty(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Make sure an empty list is non-nil.
	req, _ := http.NewRequest("GET", "/v1/connect/intentions", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionList(resp, req)
	assert.Nil(err)

	value := obj.(structs.Intentions)
	assert.NotNil(value)
	assert.Len(value, 0)
}

func TestIntentionsList_values(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create some intentions, note we create the lowest precedence first to test
	// sorting.
	for _, v := range []string{"*", "foo", "bar"} {
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceName = v

		var reply string
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/connect/intentions", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionList(resp, req)
	assert.NoError(err)

	value := obj.(structs.Intentions)
	assert.Len(value, 3)

	expected := []string{"bar", "foo", "*"}
	actual := []string{
		value[0].SourceName,
		value[1].SourceName,
		value[2].SourceName,
	}
	assert.Equal(expected, actual)
}

func TestIntentionsMatch_basic(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "*", "foo", "*"},
			{"foo", "*", "foo", "bar"},
			{"foo", "*", "foo", "baz"}, // shouldn't match
			{"foo", "*", "bar", "bar"}, // shouldn't match
			{"foo", "*", "bar", "*"},   // shouldn't match
			{"foo", "*", "*", "*"},
			{"bar", "*", "foo", "bar"}, // duplicate destination different source
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			ixn.Intention.SourceNS = v[0]
			ixn.Intention.SourceName = v[1]
			ixn.Intention.DestinationNS = v[2]
			ixn.Intention.DestinationName = v[3]

			// Create
			var reply string
			assert.Nil(a.RPC("Intention.Apply", &ixn, &reply))
		}
	}

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?by=destination&name=foo/bar", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	assert.Nil(err)

	value := obj.(map[string]structs.Intentions)
	assert.Len(value, 1)

	var actual [][]string
	expected := [][]string{
		{"bar", "*", "foo", "bar"},
		{"foo", "*", "foo", "bar"},
		{"foo", "*", "foo", "*"},
		{"foo", "*", "*", "*"},
	}
	for _, ixn := range value["foo/bar"] {
		actual = append(actual, []string{
			ixn.SourceNS,
			ixn.SourceName,
			ixn.DestinationNS,
			ixn.DestinationName,
		})
	}

	assert.Equal(expected, actual)
}

func TestIntentionsMatch_noBy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?name=foo/bar", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	assert.NotNil(err)
	assert.Contains(err.Error(), "by")
	assert.Nil(obj)
}

func TestIntentionsMatch_byInvalid(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?by=datacenter", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	assert.NotNil(err)
	assert.Contains(err.Error(), "'by' parameter")
	assert.Nil(obj)
}

func TestIntentionsMatch_noName(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?by=source", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	assert.NotNil(err)
	assert.Contains(err.Error(), "'name' not set")
	assert.Nil(obj)
}

func TestIntentionsCheck_basic(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "*", "foo", "*"},
			{"foo", "*", "foo", "bar"},
			{"bar", "*", "foo", "bar"},
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			ixn.Intention.SourceNS = v[0]
			ixn.Intention.SourceName = v[1]
			ixn.Intention.DestinationNS = v[2]
			ixn.Intention.DestinationName = v[3]
			ixn.Intention.Action = structs.IntentionActionDeny

			// Create
			var reply string
			require.Nil(a.RPC("Intention.Apply", &ixn, &reply))
		}
	}

	// Request matching intention
	{
		req, _ := http.NewRequest("GET",
			"/v1/connect/intentions/test?source=foo/bar&destination=foo/baz", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCheck(resp, req)
		require.Nil(err)
		value := obj.(*structs.IntentionQueryCheckResponse)
		require.False(value.Allowed)
	}

	// Request non-matching intention
	{
		req, _ := http.NewRequest("GET",
			"/v1/connect/intentions/test?source=foo/bar&destination=bar/qux", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCheck(resp, req)
		require.Nil(err)
		value := obj.(*structs.IntentionQueryCheckResponse)
		require.True(value.Allowed)
	}
}

func TestIntentionsCheck_noSource(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/test?destination=B", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionCheck(resp, req)
	require.NotNil(err)
	require.Contains(err.Error(), "'source' not set")
	require.Nil(obj)
}

func TestIntentionsCheck_noDestination(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/test?source=B", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionCheck(resp, req)
	require.NotNil(err)
	require.Contains(err.Error(), "'destination' not set")
	require.Nil(obj)
}

func TestIntentionsCreate_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Make sure an empty list is non-nil.
	args := structs.TestIntention(t)
	args.SourceName = "foo"
	req, _ := http.NewRequest("POST", "/v1/connect/intentions", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionCreate(resp, req)
	assert.Nil(err)

	value := obj.(intentionCreateResponse)
	assert.NotEqual("", value.ID)

	// Read the value
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: value.ID,
		}
		var resp structs.IndexedIntentions
		assert.Nil(a.RPC("Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal("foo", actual.SourceName)
	}
}

func TestIntentionsCreate_noBody(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create with no body
	req, _ := http.NewRequest("POST", "/v1/connect/intentions", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.IntentionCreate(resp, req)
	require.Error(t, err)
}

func TestIntentionsSpecificGet_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// The intention
	ixn := structs.TestIntention(t)

	// Create an intention directly
	var reply string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  ixn,
		}
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	// Get the value
	req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/connect/intentions/%s", reply), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	assert.Nil(err)

	value := obj.(*structs.Intention)
	assert.Equal(reply, value.ID)

	ixn.ID = value.ID
	ixn.RaftIndex = value.RaftIndex
	ixn.CreatedAt, ixn.UpdatedAt = value.CreatedAt, value.UpdatedAt
	assert.Equal(ixn, value)
}

func TestIntentionsSpecificGet_invalidId(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Read intention with bad ID
	req, _ := http.NewRequest("GET", "/v1/connect/intentions/hello", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	require.Nil(obj)
	require.Error(err)
	require.IsType(BadRequestError{}, err)
	require.Contains(err.Error(), "UUID")
}

func TestIntentionsSpecificUpdate_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// The intention
	ixn := structs.TestIntention(t)

	// Create an intention directly
	var reply string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  ixn,
		}
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	// Update the intention
	ixn.ID = "bogus"
	ixn.SourceName = "bar"
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/connect/intentions/%s", reply), jsonReader(ixn))
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	assert.Nil(err)

	value := obj.(intentionCreateResponse)
	assert.Equal(reply, value.ID)

	// Read the value
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		assert.Nil(a.RPC("Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal("bar", actual.SourceName)
	}
}

func TestIntentionsSpecificDelete_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// The intention
	ixn := structs.TestIntention(t)
	ixn.SourceName = "foo"

	// Create an intention directly
	var reply string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  ixn,
		}
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	// Sanity check that the intention exists
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		assert.Nil(a.RPC("Intention.Get", req, &resp))
		assert.Len(resp.Intentions, 1)
		actual := resp.Intentions[0]
		assert.Equal("foo", actual.SourceName)
	}

	// Delete the intention
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/v1/connect/intentions/%s", reply), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	assert.Nil(err)
	assert.Equal(true, obj)

	// Verify the intention is gone
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		err := a.RPC("Intention.Get", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), "not found")
	}
}

func TestParseIntentionMatchEntry(t *testing.T) {
	cases := []struct {
		Input    string
		Expected structs.IntentionMatchEntry
		Err      bool
	}{
		{
			"foo",
			structs.IntentionMatchEntry{
				Namespace: structs.IntentionDefaultNamespace,
				Name:      "foo",
			},
			false,
		},

		{
			"foo/bar",
			structs.IntentionMatchEntry{
				Namespace: "foo",
				Name:      "bar",
			},
			false,
		},

		{
			"foo/bar/baz",
			structs.IntentionMatchEntry{},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Input, func(t *testing.T) {
			assert := assert.New(t)
			actual, err := parseIntentionMatchEntry(tc.Input)
			assert.Equal(err != nil, tc.Err, err)
			if err != nil {
				return
			}

			assert.Equal(tc.Expected, actual)
		})
	}
}
