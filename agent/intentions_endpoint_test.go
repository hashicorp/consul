package agent

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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
			Intention:  &structs.Intention{SourceName: v},
		}
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
