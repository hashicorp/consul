package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

func TestIntentionsList_empty(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Make sure an empty list is non-nil.
	req, _ := http.NewRequest("GET", "/v1/connect/intentions", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionList(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	value := obj.(structs.Intentions)
	if value == nil || len(value) != 0 {
		t.Fatalf("bad: %v", value)
	}
}

func TestIntentionsList_values(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create some intentions
	for _, v := range []string{"foo", "bar"} {
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceName = v

		var reply string
		if err := a.RPC("Intention.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Request
	req, _ := http.NewRequest("GET", "/v1/connect/intentions", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionList(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	value := obj.(structs.Intentions)
	if len(value) != 2 {
		t.Fatalf("bad: %v", value)
	}

	expected := []string{"bar", "foo"}
	actual := []string{value[0].SourceName, value[1].SourceName}
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestIntentionsMatch_basic(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create some intentions
	{
		insert := [][]string{
			{"foo", "*"},
			{"foo", "bar"},
			{"foo", "baz"}, // shouldn't match
			{"bar", "bar"}, // shouldn't match
			{"bar", "*"},   // shouldn't match
			{"*", "*"},
		}

		for _, v := range insert {
			ixn := structs.IntentionRequest{
				Datacenter: "dc1",
				Op:         structs.IntentionOpCreate,
				Intention:  structs.TestIntention(t),
			}
			ixn.Intention.DestinationNS = v[0]
			ixn.Intention.DestinationName = v[1]

			// Create
			var reply string
			if err := a.RPC("Intention.Apply", &ixn, &reply); err != nil {
				t.Fatalf("err: %v", err)
			}
		}
	}

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?by=destination&name=foo/bar", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	value := obj.(map[string]structs.Intentions)
	if len(value) != 1 {
		t.Fatalf("bad: %v", value)
	}

	var actual [][]string
	expected := [][]string{{"foo", "bar"}, {"foo", "*"}, {"*", "*"}}
	for _, ixn := range value["foo/bar"] {
		actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad (got, wanted):\n\n%#v\n\n%#v", actual, expected)
	}
}

func TestIntentionsMatch_noBy(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?name=foo/bar", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	if err == nil || !strings.Contains(err.Error(), "by") {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatal("should have no response")
	}
}

func TestIntentionsMatch_byInvalid(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?by=datacenter", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	if err == nil || !strings.Contains(err.Error(), "'by' parameter") {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatal("should have no response")
	}
}

func TestIntentionsMatch_noName(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Request
	req, _ := http.NewRequest("GET",
		"/v1/connect/intentions/match?by=source", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionMatch(resp, req)
	if err == nil || !strings.Contains(err.Error(), "'name' not set") {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatal("should have no response")
	}
}

func TestIntentionsCreate_good(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Make sure an empty list is non-nil.
	args := structs.TestIntention(t)
	args.SourceName = "foo"
	req, _ := http.NewRequest("POST", "/v1/connect/intentions", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	value := obj.(intentionCreateResponse)
	if value.ID == "" {
		t.Fatalf("bad: %v", value)
	}

	// Read the value
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: value.ID,
		}
		var resp structs.IndexedIntentions
		if err := a.RPC("Intention.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(resp.Intentions) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Intentions[0]
		if actual.SourceName != "foo" {
			t.Fatalf("bad: %#v", actual)
		}
	}
}

func TestIntentionsSpecificGet_good(t *testing.T) {
	t.Parallel()

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
		if err := a.RPC("Intention.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Get the value
	req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/connect/intentions/%s", reply), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	value := obj.(*structs.Intention)
	if value.ID != reply {
		t.Fatalf("bad: %v", value)
	}

	ixn.ID = value.ID
	ixn.RaftIndex = value.RaftIndex
	ixn.CreatedAt, ixn.UpdatedAt = value.CreatedAt, value.UpdatedAt
	if !reflect.DeepEqual(value, ixn) {
		t.Fatalf("bad (got, want):\n\n%#v\n\n%#v", value, ixn)
	}
}

func TestIntentionsSpecificUpdate_good(t *testing.T) {
	t.Parallel()

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
		if err := a.RPC("Intention.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Update the intention
	ixn.ID = "bogus"
	ixn.SourceName = "bar"
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/connect/intentions/%s", reply), jsonReader(ixn))
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("obj should be nil: %v", err)
	}

	// Read the value
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		if err := a.RPC("Intention.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(resp.Intentions) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Intentions[0]
		if actual.SourceName != "bar" {
			t.Fatalf("bad: %#v", actual)
		}
	}
}

func TestIntentionsSpecificDelete_good(t *testing.T) {
	t.Parallel()

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
		if err := a.RPC("Intention.Apply", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Sanity check that the intention exists
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		if err := a.RPC("Intention.Get", req, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(resp.Intentions) != 1 {
			t.Fatalf("bad: %v", resp)
		}
		actual := resp.Intentions[0]
		if actual.SourceName != "foo" {
			t.Fatalf("bad: %#v", actual)
		}
	}

	// Delete the intention
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/v1/connect/intentions/%s", reply), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.IntentionSpecific(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("obj should be nil: %v", err)
	}

	// Verify the intention is gone
	{
		req := &structs.IntentionQueryRequest{
			Datacenter:  "dc1",
			IntentionID: reply,
		}
		var resp structs.IndexedIntentions
		err := a.RPC("Intention.Get", req, &resp)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err: %v", err)
		}
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
			actual, err := parseIntentionMatchEntry(tc.Input)
			if (err != nil) != tc.Err {
				t.Fatalf("err: %s", err)
			}
			if err != nil {
				return
			}

			if !reflect.DeepEqual(actual, tc.Expected) {
				t.Fatalf("bad: %#v", actual)
			}
		})
	}
}
