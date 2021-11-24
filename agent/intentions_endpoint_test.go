package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestIntentionList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("empty", func(t *testing.T) {
		// Make sure an empty list is non-nil.
		req, _ := http.NewRequest("GET", "/v1/connect/intentions", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionList(resp, req)
		require.NoError(t, err)

		value := obj.(structs.Intentions)
		require.NotNil(t, value)
		require.Len(t, value, 0)
	})

	t.Run("values", func(t *testing.T) {
		// Create some intentions, note we create the lowest precedence first to test
		// sorting.
		//
		// Also create one non-legacy one using a different destination.
		var ids []string
		for _, v := range []string{"*", "foo", "bar", "zim"} {
			req := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			req.Intention.SourceName = v

			if v == "zim" {
				req.Op = structs.IntentionOpUpsert // non-legacy
				req.Intention.DestinationName = "gir"
			}

			var reply string
			require.NoError(t, a.RPC("Intention.Apply", &req, &reply))
			ids = append(ids, reply)
		}

		// Request
		req, err := http.NewRequest("GET", "/v1/connect/intentions", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionList(resp, req)
		require.NoError(t, err)

		value := obj.(structs.Intentions)
		require.Len(t, value, 4)

		require.Equal(t, []string{"bar->db", "foo->db", "zim->gir", "*->db"},
			[]string{
				value[0].SourceName + "->" + value[0].DestinationName,
				value[1].SourceName + "->" + value[1].DestinationName,
				value[2].SourceName + "->" + value[2].DestinationName,
				value[3].SourceName + "->" + value[3].DestinationName,
			})
		require.Equal(t, []string{ids[2], ids[1], "", ids[0]},
			[]string{
				value[0].ID,
				value[1].ID,
				value[2].ID,
				value[3].ID,
			})
	})
}

func TestIntentionMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some intentions
	{
		insert := [][]string{
			{"default", "*", "default", "*"},
			{"default", "*", "default", "bar"},
			{"default", "*", "default", "baz"}, // shouldn't match
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

			if ixn.Intention.DestinationName == "baz" {
				// make the "baz" destination be non-legacy
				ixn.Op = structs.IntentionOpUpsert
			}

			// Create
			var reply string
			require.NoError(t, a.RPC("Intention.Apply", &ixn, &reply))
		}
	}

	t.Run("no by", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/match?name=foo/bar", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionMatch(resp, req)
		testutil.RequireErrorContains(t, err, "by")
		require.Nil(t, obj)
	})

	t.Run("by invalid", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/match?by=datacenter", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionMatch(resp, req)
		testutil.RequireErrorContains(t, err, "'by' parameter")
		require.Nil(t, obj)
	})

	t.Run("no name", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/match?by=source", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionMatch(resp, req)
		testutil.RequireErrorContains(t, err, "'name' not set")
		require.Nil(t, obj)
	})

	t.Run("success", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/match?by=destination&name=bar", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionMatch(resp, req)
		require.NoError(t, err)

		value := obj.(map[string]structs.Intentions)
		require.Len(t, value, 1)

		var actual [][]string
		expected := [][]string{
			{"default", "*", "default", "bar"},
			{"default", "*", "default", "*"},
		}
		for _, ixn := range value["bar"] {
			actual = append(actual, []string{
				ixn.SourceNS,
				ixn.SourceName,
				ixn.DestinationNS,
				ixn.DestinationName,
			})
		}

		require.Equal(t, expected, actual)
	})

	t.Run("success with cache", func(t *testing.T) {
		// First request is a MISS, but it primes the cache for the second attempt
		for i := 0; i < 2; i++ {
			req, err := http.NewRequest("GET", "/v1/connect/intentions/match?by=destination&name=bar&cached", nil)
			require.NoError(t, err)

			resp := httptest.NewRecorder()
			obj, err := a.srv.IntentionMatch(resp, req)
			require.NoError(t, err)

			// The GET request primes the cache so the POST is a hit.
			if i == 0 {
				// Should be a cache miss
				require.Equal(t, "MISS", resp.Header().Get("X-Cache"))
			} else {
				// Should be a cache HIT now!
				require.Equal(t, "HIT", resp.Header().Get("X-Cache"))
			}

			value := obj.(map[string]structs.Intentions)
			require.Len(t, value, 1)

			var actual [][]string
			expected := [][]string{
				{"default", "*", "default", "bar"},
				{"default", "*", "default", "*"},
			}
			for _, ixn := range value["bar"] {
				actual = append(actual, []string{
					ixn.SourceNS,
					ixn.SourceName,
					ixn.DestinationNS,
					ixn.DestinationName,
				})
			}

			require.Equal(t, expected, actual)
		}
	})
}

func TestIntentionCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some intentions
	{
		insert := [][]string{
			{"default", "*", "default", "baz"},
			{"default", "*", "default", "bar"},
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

			if ixn.Intention.DestinationName == "baz" {
				// make the "baz" destination be non-legacy
				ixn.Op = structs.IntentionOpUpsert
			}

			// Create
			var reply string
			require.NoError(t, a.RPC("Intention.Apply", &ixn, &reply))
		}
	}

	t.Run("no source", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/test?destination=B", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCheck(resp, req)
		testutil.RequireErrorContains(t, err, "'source' not set")
		require.Nil(t, obj)
	})

	t.Run("no destination", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/test?source=B", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCheck(resp, req)
		testutil.RequireErrorContains(t, err, "'destination' not set")
		require.Nil(t, obj)
	})

	t.Run("success - matching intention", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/test?source=bar&destination=baz", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCheck(resp, req)
		require.NoError(t, err)
		value := obj.(*structs.IntentionQueryCheckResponse)
		require.False(t, value.Allowed)
	})

	t.Run("success - non-matching intention", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/v1/connect/intentions/test?source=bar&destination=qux", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCheck(resp, req)
		require.NoError(t, err)
		value := obj.(*structs.IntentionQueryCheckResponse)
		require.True(t, value.Allowed)
	})
}

func TestIntentionPutExact(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("no body", func(t *testing.T) {
		// Create with no body
		req, err := http.NewRequest("PUT", "/v1/connect/intentions", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		_, err = a.srv.IntentionExact(resp, req)
		require.Error(t, err)
	})

	t.Run("source is required", func(t *testing.T) {
		ixn := structs.TestIntention(t)
		ixn.SourceName = "foo"
		req, err := http.NewRequest("PUT", "/v1/connect/intentions?source=&destination=db", jsonReader(ixn))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		_, err = a.srv.IntentionExact(resp, req)
		require.Error(t, err)
	})

	t.Run("destination is required", func(t *testing.T) {
		ixn := structs.TestIntention(t)
		ixn.SourceName = "foo"
		req, err := http.NewRequest("PUT", "/v1/connect/intentions?source=foo&destination=", jsonReader(ixn))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		_, err = a.srv.IntentionExact(resp, req)
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		ixn := structs.TestIntention(t)
		ixn.SourceName = "foo"
		req, err := http.NewRequest("PUT", "/v1/connect/intentions?source=foo&destination=db", jsonReader(ixn))
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionExact(resp, req)
		require.NoError(t, err)
		require.True(t, obj.(bool))

		// Read the value
		{
			req := &structs.IntentionQueryRequest{
				Datacenter: "dc1",
				Exact:      ixn.ToExact(),
			}

			var resp structs.IndexedIntentions
			require.NoError(t, a.RPC("Intention.Get", req, &resp))
			require.Len(t, resp.Intentions, 1)
			actual := resp.Intentions[0]
			require.Equal(t, "foo", actual.SourceName)
			require.Empty(t, actual.ID) // new style
		}
	})
}

func TestIntentionCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("no body", func(t *testing.T) {
		// Create with no body
		req, _ := http.NewRequest("POST", "/v1/connect/intentions", nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.IntentionCreate(resp, req)
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		// Make sure an empty list is non-nil.
		args := structs.TestIntention(t)
		args.SourceName = "foo"
		req, _ := http.NewRequest("POST", "/v1/connect/intentions", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionCreate(resp, req)
		require.NoError(t, err)

		value := obj.(intentionCreateResponse)
		require.NotEmpty(t, value.ID)

		// Read the value
		{
			req := &structs.IntentionQueryRequest{
				Datacenter:  "dc1",
				IntentionID: value.ID,
			}
			var resp structs.IndexedIntentions
			require.NoError(t, a.RPC("Intention.Get", req, &resp))
			require.Len(t, resp.Intentions, 1)
			actual := resp.Intentions[0]
			require.Equal(t, "foo", actual.SourceName)
		}
	})

	t.Run("partition rejected", func(t *testing.T) {
		{
			args := structs.TestIntention(t)
			args.SourcePartition = "part1"
			req, _ := http.NewRequest("POST", "/v1/connect/intentions", jsonReader(args))
			resp := httptest.NewRecorder()
			_, err := a.srv.IntentionCreate(resp, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "Cannot specify a source partition")
		}
		{
			args := structs.TestIntention(t)
			args.DestinationPartition = "part2"
			req, _ := http.NewRequest("POST", "/v1/connect/intentions", jsonReader(args))
			resp := httptest.NewRecorder()
			_, err := a.srv.IntentionCreate(resp, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "Cannot specify a destination partition")
		}
	})
}

func TestIntentionSpecificGet(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ixn := structs.TestIntention(t)

	// Create an intention directly
	var reply string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  ixn,
		}
		require.NoError(t, a.RPC("Intention.Apply", &req, &reply))
	}

	t.Run("invalid id", func(t *testing.T) {
		// Read intention with bad ID
		req, _ := http.NewRequest("GET", "/v1/connect/intentions/hello", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionSpecific(resp, req)
		require.Nil(t, obj)
		require.Error(t, err)
		require.IsType(t, BadRequestError{}, err)
		require.Contains(t, err.Error(), "UUID")
	})

	t.Run("success", func(t *testing.T) {
		// Get the value
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/connect/intentions/%s", reply), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionSpecific(resp, req)
		require.NoError(t, err)

		value := obj.(*structs.Intention)
		require.Equal(t, reply, value.ID)

		ixn.ID = value.ID
		ixn.Precedence = value.Precedence
		ixn.RaftIndex = value.RaftIndex
		ixn.Hash = value.Hash
		ixn.CreatedAt, ixn.UpdatedAt = value.CreatedAt, value.UpdatedAt
		require.Equal(t, ixn, value)
	})
}

func TestIntentionSpecificUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
		require.NoError(t, a.RPC("Intention.Apply", &req, &reply))
	}

	// Update the intention
	ixn.ID = "bogus"
	ixn.SourceName = "bar"
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/connect/intentions/%s", reply), jsonReader(ixn))
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	require.NoError(t, err)

	value := obj.(intentionCreateResponse)
	require.Equal(t, reply, value.ID)

	// Read the value
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		require.NoError(t, a.RPC("Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, "bar", actual.SourceName)
	}

	t.Run("partitions rejected", func(t *testing.T) {
		{
			ixn.DestinationPartition = "part1"
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/connect/intentions/%s", reply), jsonReader(ixn))
			resp := httptest.NewRecorder()
			_, err := a.srv.IntentionSpecific(resp, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "Cannot specify a destination partition")
		}
		{
			ixn.DestinationPartition = "default"
			ixn.SourcePartition = "part2"
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/connect/intentions/%s", reply), jsonReader(ixn))
			resp := httptest.NewRecorder()
			_, err := a.srv.IntentionSpecific(resp, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "Cannot specify a source partition")
		}
	})
}

func TestIntentionDeleteExact(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ixn := structs.TestIntention(t)
	ixn.SourceName = "foo"

	t.Run("cannot delete non-existent intention", func(t *testing.T) {
		// Delete the intention
		req, err := http.NewRequest("DELETE", "/v1/connect/intentions/exact?source=foo&destination=db", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionExact(resp, req)
		require.NoError(t, err) // by-name deletions are idempotent
		require.Equal(t, true, obj)
	})

	exact := ixn.ToExact()

	// Create an intention directly
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpUpsert,
			Intention:  ixn,
		}
		var ignored string
		require.NoError(t, a.RPC("Intention.Apply", &req, &ignored))
	}

	// Sanity check that the intention exists
	{
		req := &structs.IntentionQueryRequest{
			Datacenter: "dc1",
			Exact:      exact,
		}
		var resp structs.IndexedIntentions
		require.NoError(t, a.RPC("Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, "foo", actual.SourceName)
		require.Empty(t, actual.ID) // new style
	}

	t.Run("source is required", func(t *testing.T) {
		// Delete the intention
		req, err := http.NewRequest("DELETE", "/v1/connect/intentions/exact?source=&destination=db", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		_, err = a.srv.IntentionExact(resp, req)
		require.Error(t, err)
	})

	t.Run("destination is required", func(t *testing.T) {
		// Delete the intention
		req, err := http.NewRequest("DELETE", "/v1/connect/intentions/exact?source=foo&destination=", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		_, err = a.srv.IntentionExact(resp, req)
		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		// Delete the intention
		req, err := http.NewRequest("DELETE", "/v1/connect/intentions/exact?source=foo&destination=db", nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionExact(resp, req)
		require.NoError(t, err)
		require.Equal(t, true, obj)

		// Verify the intention is gone
		{
			req := &structs.IntentionQueryRequest{
				Datacenter: "dc1",
				Exact:      exact,
			}
			var resp structs.IndexedIntentions
			err := a.RPC("Intention.Get", req, &resp)
			testutil.RequireErrorContains(t, err, "not found")
		}
	})
}

func TestIntentionSpecificDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// The intention
	ixn := structs.TestIntention(t)
	ixn.SourceName = "foo"

	t.Run("cannot delete non-existent intention", func(t *testing.T) {
		fakeID := generateUUID()

		req, err := http.NewRequest("DELETE", "/v1/connect/intentions/"+fakeID, nil)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := a.srv.IntentionSpecific(resp, req)
		testutil.RequireErrorContains(t, err, "Cannot delete non-existent intention")
		require.Nil(t, obj)
	})

	// Create an intention directly
	var reply string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  ixn,
		}
		require.NoError(t, a.RPC("Intention.Apply", &req, &reply))
	}

	// Sanity check that the intention exists
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		require.NoError(t, a.RPC("Intention.Get", req, &resp))
		require.Len(t, resp.Intentions, 1)
		actual := resp.Intentions[0]
		require.Equal(t, "foo", actual.SourceName)
	}

	// Delete the intention
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/v1/connect/intentions/%s", reply), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	require.NoError(t, err)
	require.Equal(t, true, obj)

	// Verify the intention is gone
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		err := a.RPC("Intention.Get", req, &resp)
		testutil.RequireErrorContains(t, err, "not found")
	}
}

func TestParseIntentionStringComponent(t *testing.T) {
	cases := []struct {
		TestName     string
		Input        string
		ExpectedAP   string
		ExpectedNS   string
		ExpectedName string
		Err          bool
	}{
		{
			TestName:     "single name",
			Input:        "foo",
			ExpectedAP:   "",
			ExpectedNS:   "",
			ExpectedName: "foo",
		},
		{
			TestName:     "namespace and name",
			Input:        "foo/bar",
			ExpectedAP:   "",
			ExpectedNS:   "foo",
			ExpectedName: "bar",
		},
		{
			TestName:     "partition, namespace, and name",
			Input:        "foo/bar/baz",
			ExpectedAP:   "foo",
			ExpectedNS:   "bar",
			ExpectedName: "baz",
		},
		{
			TestName:     "empty ns",
			Input:        "/bar",
			ExpectedAP:   "",
			ExpectedNS:   "",
			ExpectedName: "bar",
		},
		{
			TestName: "invalid input",
			Input:    "uhoh/blah/foo/bar",
			Err:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.TestName, func(t *testing.T) {
			var entMeta structs.EnterpriseMeta
			ap, ns, name, err := parseIntentionStringComponent(tc.Input, &entMeta)
			if tc.Err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				assert.Equal(t, tc.ExpectedAP, ap)
				assert.Equal(t, tc.ExpectedNS, ns)
				assert.Equal(t, tc.ExpectedName, name)
			}
		})
	}
}
