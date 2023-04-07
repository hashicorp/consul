package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/serf/coordinate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestCoordinate_Disabled_Response(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, `
		disable_coordinates = true
`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	tests := []func(resp http.ResponseWriter, req *http.Request) (interface{}, error){
		a.srv.CoordinateDatacenters,
		a.srv.CoordinateNodes,
		a.srv.CoordinateNode,
		a.srv.CoordinateUpdate,
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/should/not/care", nil)
			resp := httptest.NewRecorder()
			obj, err := tt(resp, req)
			if httpErr, ok := err.(HTTPError); ok {
				if httpErr.StatusCode != 401 {
					t.Fatalf("expected status 401 but got %d", httpErr.StatusCode)
				}
			} else {
				t.Fatalf("expected HTTP error but got %v", err)
			}

			if obj != nil {
				t.Fatalf("bad: %#v", obj)
			}
			if !strings.Contains(err.Error(), "Coordinate support disabled") {
				t.Fatalf("bad: %#v", resp)
			}
		})
	}
}

func TestCoordinate_Datacenters(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/coordinate/datacenters", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CoordinateDatacenters(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Fatalf("bad: %v", resp.Code)
	}

	maps := obj.([]structs.DatacenterMap)
	if len(maps) != 1 ||
		maps[0].Datacenter != "dc1" ||
		len(maps[0].Coordinates) != 1 ||
		maps[0].Coordinates[0].Node != a.Config.NodeName {
		t.Fatalf("bad: %v", maps)
	}
}

func TestCoordinate_Nodes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Make sure an empty list is non-nil.
	req, _ := http.NewRequest("GET", "/v1/coordinate/nodes?dc=dc1", nil)
	resp := httptest.NewRecorder()
	retry.Run(t, func(r *retry.R) {
		obj, err := a.srv.CoordinateNodes(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		if resp.Code != http.StatusOK {
			r.Fatalf("bad: %v", resp.Code)
		}

		// Check that coordinates are empty before registering a node
		coordinates, ok := obj.(structs.Coordinates)
		if !ok {
			r.Fatalf("expected: structs.Coordinates, received: %+v", obj)
		}

		if len(coordinates) != 0 {
			r.Fatalf("coordinates should be empty, received: %v", coordinates)
		}
	})

	// Register the nodes.
	nodes := []string{"foo", "bar"}
	for _, node := range nodes {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       node,
			Address:    "127.0.0.1",
		}
		var reply struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Send some coordinates for a few nodes, waiting a little while for the
	// batch update to run.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	var out struct{}
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Query back and check the nodes are present and sorted correctly.
	req, _ = http.NewRequest("GET", "/v1/coordinate/nodes?dc=dc1", nil)
	resp = httptest.NewRecorder()
	retry.Run(t, func(r *retry.R) {
		obj, err := a.srv.CoordinateNodes(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		if resp.Code != http.StatusOK {
			r.Fatalf("bad: %v", resp.Code)
		}

		coordinates, ok := obj.(structs.Coordinates)
		if !ok {
			r.Fatalf("expected: structs.Coordinates, received: %+v", obj)
		}
		if len(coordinates) != 2 ||
			coordinates[0].Node != "bar" ||
			coordinates[1].Node != "foo" {
			r.Fatalf("expected: bar, foo received: %v", coordinates)
		}
	})
}

func TestCoordinate_Node(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Make sure we get a 404 with no coordinates.
	req, _ := http.NewRequest("GET", "/v1/coordinate/node/foo?dc=dc1", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.CoordinateNode(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if resp.Code != http.StatusNotFound {
		t.Fatalf("bad: %v", resp.Code)
	}

	// Register the nodes.
	nodes := []string{"foo", "bar"}
	for _, node := range nodes {
		req := structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       node,
			Address:    "127.0.0.1",
		}
		var reply struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", &req, &reply); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Send some coordinates for a few nodes, waiting a little while for the
	// batch update to run.
	arg1 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	var out struct{}
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg1, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg2 := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg2, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Query back and check the nodes are present.
	req, _ = http.NewRequest("GET", "/v1/coordinate/node/foo?dc=dc1", nil)
	resp = httptest.NewRecorder()
	obj, err := a.srv.CoordinateNode(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Fatalf("bad: %v", resp.Code)
	}

	coordinates := obj.(structs.Coordinates)
	if len(coordinates) != 1 ||
		coordinates[0].Node != "foo" {
		t.Fatalf("bad: %v", coordinates)
	}
}

func TestCoordinate_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register the node.
	reg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var reply struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", &reg, &reply); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Update the coordinates and wait for it to complete.
	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = -5.0
	body := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coord,
	}
	req, _ := http.NewRequest("PUT", "/v1/coordinate/update", jsonReader(body))
	resp := httptest.NewRecorder()
	_, err := a.srv.CoordinateUpdate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Fatalf("bad: %v", resp.Code)
	}

	time.Sleep(300 * time.Millisecond)

	// Query back and check the coordinates are present.
	args := structs.NodeSpecificRequest{Node: "foo", Datacenter: "dc1"}
	var coords structs.IndexedCoordinates
	if err := a.RPC(context.Background(), "Coordinate.Node", &args, &coords); err != nil {
		t.Fatalf("err: %s", err)
	}

	coordinates := coords.Coordinates
	if len(coordinates) != 1 ||
		coordinates[0].Node != "foo" {
		t.Fatalf("bad: %v", coordinates)
	}
}

func TestCoordinate_Update_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Height = -5.0
	body := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coord,
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/coordinate/update", jsonReader(body))
		if _, err := a.srv.CoordinateUpdate(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/coordinate/update", jsonReader(body))
		req.Header.Add("X-Consul-Token", "root")
		if _, err := a.srv.CoordinateUpdate(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}
