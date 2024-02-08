// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package catalogv2

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/test-integ/topoutil"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

type testCase struct {
	permissions []*permission
	result      []*testResult
}

type permission struct {
	allow                bool
	excludeSource        bool
	includeSourceTenancy bool
	excludeSourceTenancy bool
	destRules            []*destRules
}

type destRules struct {
	values   *ruleValues
	excludes []*ruleValues
}

type ruleValues struct {
	portNames []string
	path      string
	pathPref  string
	pathReg   string
	headers   []string
	methods   []string
}

type testResult struct {
	fail    bool
	port    string
	path    string
	headers map[string]string
}

func newTrafficPermissions(p *permission, srcTenancy *pbresource.Tenancy) *pbauth.TrafficPermissions {
	sources := []*pbauth.Source{{
		IdentityName: "static-client",
		Namespace:    srcTenancy.Namespace,
		Partition:    srcTenancy.Partition,
	}}
	destinationRules := []*pbauth.DestinationRule{}
	if p != nil {
		srcId := "static-client"
		if p.includeSourceTenancy {
			srcId = ""
		}
		if p.excludeSource {
			sources = []*pbauth.Source{{
				IdentityName: srcId,
				Namespace:    srcTenancy.Namespace,
				Partition:    srcTenancy.Partition,
				Exclude: []*pbauth.ExcludeSource{{
					IdentityName: "static-client",
					Namespace:    srcTenancy.Namespace,
					Partition:    srcTenancy.Partition,
				}},
			}}
		} else {
			sources = []*pbauth.Source{{
				IdentityName: srcId,
				Namespace:    srcTenancy.Namespace,
				Partition:    srcTenancy.Partition,
			}}
		}
		for _, dr := range p.destRules {
			destRule := &pbauth.DestinationRule{}
			if dr.values != nil {
				destRule.PathExact = dr.values.path
				destRule.PathPrefix = dr.values.pathPref
				destRule.PathRegex = dr.values.pathReg
				destRule.Methods = dr.values.methods
				destRule.PortNames = dr.values.portNames
				destRule.Headers = []*pbauth.DestinationRuleHeader{}
				for _, h := range dr.values.headers {
					destRule.Headers = append(destRule.Headers, &pbauth.DestinationRuleHeader{
						Name:    h,
						Present: true,
					})
				}
			}
			var excludePermissions []*pbauth.ExcludePermissionRule
			for _, e := range dr.excludes {
				eRule := &pbauth.ExcludePermissionRule{
					PathExact:  e.path,
					PathPrefix: e.pathPref,
					PathRegex:  e.pathReg,
					Methods:    e.methods,
					PortNames:  e.portNames,
				}
				eRule.Headers = []*pbauth.DestinationRuleHeader{}
				for _, h := range e.headers {
					eRule.Headers = append(eRule.Headers, &pbauth.DestinationRuleHeader{
						Name:    h,
						Present: true,
					})
				}
				excludePermissions = append(excludePermissions, eRule)
			}
			destRule.Exclude = excludePermissions
			destinationRules = append(destinationRules, destRule)
		}
	}
	action := pbauth.Action_ACTION_ALLOW
	if !p.allow {
		action = pbauth.Action_ACTION_DENY
	}
	return &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "static-server",
		},
		Action: action,
		Permissions: []*pbauth.Permission{{
			Sources:          sources,
			DestinationRules: destinationRules,
		}},
	}

}

// This tests runs a gauntlet of traffic permissions updates and validates that the request status codes match the intended rules
func TestL7TrafficPermissions(t *testing.T) {
	testcases := map[string]testCase{
		// L4 permissions
		"basic":                       {permissions: []*permission{{allow: true}}, result: []*testResult{{fail: false}}},
		"client-exclude":              {permissions: []*permission{{allow: true, includeSourceTenancy: true, excludeSource: true}}, result: []*testResult{{fail: true}}},
		"allow-all-client-in-tenancy": {permissions: []*permission{{allow: true, includeSourceTenancy: true}}, result: []*testResult{{fail: false}}},
		"only-one-port":               {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{portNames: []string{"http"}}}}}}, result: []*testResult{{fail: true, port: "http2"}}},
		"exclude-port":                {permissions: []*permission{{allow: true, destRules: []*destRules{{excludes: []*ruleValues{{portNames: []string{"http"}}}}}}}, result: []*testResult{{fail: true, port: "http"}}},
		// L7 permissions
		"methods": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{methods: []string{"POST", "PUT", "PATCH", "DELETE", "CONNECT", "HEAD", "OPTIONS", "TRACE"}, pathPref: "/"}}}}},
			// fortio fetch2 is configured to GET
			result: []*testResult{{fail: true}}},
		"headers": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{headers: []string{"a", "b"}, pathPref: "/"}}}}},
			result: []*testResult{{fail: true}, {fail: true, headers: map[string]string{"a": "1"}}, {fail: false, headers: map[string]string{"a": "1", "b": "2"}}}},
		"path-prefix-all": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/", methods: []string{"GET"}}}}}}, result: []*testResult{{fail: false}}},
		"method-exclude": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/"}, excludes: []*ruleValues{{methods: []string{"GET"}}}}}}},
			// fortio fetch2 is configured to GET
			result: []*testResult{{fail: true}}},
		"exclude-paths-and-headers": {permissions: []*permission{{allow: true, destRules: []*destRules{
			{
				values:   &ruleValues{pathPref: "/f", headers: []string{"a"}},
				excludes: []*ruleValues{{headers: []string{"b"}, path: "/foobar"}},
			}}}},
			result: []*testResult{
				{fail: false, path: "foobar", headers: map[string]string{"a": "1"}},
				{fail: false, path: "foo", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: true, path: "foobar", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: false, path: "foo", headers: map[string]string{"a": "1"}},
				{fail: true, path: "foo", headers: map[string]string{"b": "2"}},
				{fail: true, path: "baz", headers: map[string]string{"a": "1"}},
			}},
		"exclude-paths-or-headers": {permissions: []*permission{{allow: true, destRules: []*destRules{
			{values: &ruleValues{pathPref: "/f", headers: []string{"a"}}, excludes: []*ruleValues{{headers: []string{"b"}}, {path: "/foobar"}}}}}},
			result: []*testResult{
				{fail: true, path: "foobar", headers: map[string]string{"a": "1"}},
				{fail: true, path: "foo", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: true, path: "foobar", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: false, path: "foo", headers: map[string]string{"a": "1"}},
				{fail: false, path: "foo", headers: map[string]string{"a": "1"}},
				{fail: true, path: "baz", port: "http", headers: map[string]string{"a": "1"}},
			}},
		"path-or-header": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/bar"}}, {values: &ruleValues{headers: []string{"b"}}}}}},
			result: []*testResult{
				{fail: false, path: "bar"},
				{fail: false, path: "foo", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: false, path: "bar", headers: map[string]string{"b": "2"}},
				{fail: true, path: "foo", headers: map[string]string{"a": "1"}},
			}},
		"path-and-header": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/bar", headers: []string{"b"}}}}}},
			result: []*testResult{
				{fail: true, path: "bar"},
				{fail: true, path: "foo", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: false, path: "bar", headers: map[string]string{"b": "2"}},
				{fail: true, path: "foo", headers: map[string]string{"a": "1"}},
			}},
		"path-regex-exclude": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/"}, excludes: []*ruleValues{{pathReg: ".*dns.*"}}}}}},
			result: []*testResult{{fail: true, path: "fortio/rest/dns"}, {fail: false, path: "fortio/rest/status"}}},
		"header-include-exclude-by-port": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/", headers: []string{"experiment1", "experiment2"}}, excludes: []*ruleValues{{portNames: []string{"http2"}, headers: []string{"experiment1"}}}}}}},
			result: []*testResult{{fail: true, port: "http2", headers: map[string]string{"experiment1": "a", "experiment2": "b"}},
				{fail: false, port: "http", headers: map[string]string{"experiment1": "a", "experiment2": "b"}},
				{fail: true, port: "http2", headers: map[string]string{"experiment2": "b"}},
				{fail: true, port: "http", headers: map[string]string{"experiment3": "c"}},
			}},
		"two-tp-or": {permissions: []*permission{{allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/bar"}}}}, {allow: true, destRules: []*destRules{{values: &ruleValues{headers: []string{"b"}}}}}},
			result: []*testResult{
				{fail: false, path: "bar"},
				{fail: false, path: "foo", headers: map[string]string{"a": "1", "b": "2"}},
				{fail: false, path: "bar", headers: map[string]string{"b": "2"}},
				{fail: true, path: "foo", headers: map[string]string{"a": "1"}},
			}},
	}
	if utils.IsEnterprise() {
		// DENY and ALLOW permissions
		testcases["deny-cancel-allow"] = testCase{permissions: []*permission{{allow: true}, {allow: false}}, result: []*testResult{{fail: true}}}
		testcases["l4-deny-l7-allow"] = testCase{permissions: []*permission{{allow: false}, {allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/"}}}}}, result: []*testResult{{fail: true}, {fail: true, path: "test"}}}
		testcases["l7-deny-l4-allow"] = testCase{permissions: []*permission{{allow: true}, {allow: true, destRules: []*destRules{{values: &ruleValues{pathPref: "/"}}}}, {allow: false, destRules: []*destRules{{values: &ruleValues{pathPref: "/foo"}}}}},
			result: []*testResult{{fail: false}, {fail: false, path: "test"}, {fail: true, path: "foo-bar"}}}
	}

	tenancies := []*pbresource.Tenancy{
		{
			Partition: "default",
			Namespace: "default",
		},
	}
	if utils.IsEnterprise() {
		tenancies = append(tenancies, &pbresource.Tenancy{
			Partition: "ap1",
			Namespace: "ns1",
		})
	}
	cfg := testL7TrafficPermissionsCreator{tenancies}.NewConfig(t)
	targetImage := utils.TargetImages()
	imageName := targetImage.Consul
	if utils.IsEnterprise() {
		imageName = targetImage.ConsulEnterprise
	}
	t.Log("running with target image: " + imageName)

	sp := sprawltest.Launch(t, cfg)

	asserter := topoutil.NewAsserter(sp)

	topo := sp.Topology()
	cluster := topo.Clusters["dc1"]
	ships := topo.ComputeRelationships()

	clientV2 := sp.ResourceServiceClientForCluster(cluster.Name)

	// Make sure services exist
	for _, tenancy := range tenancies {
		for _, name := range []string{
			"static-server",
			"static-client",
		} {
			libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, name, tenancy, len(tenancies))
		}
	}
	var initialTrafficPerms []*pbresource.Resource
	for testName, tc := range testcases {
		// Delete old TP and write new one for a new test case
		mustDeleteTestResources(t, clientV2, initialTrafficPerms)
		initialTrafficPerms = []*pbresource.Resource{}
		for _, st := range tenancies {
			for _, dt := range tenancies {
				for i, p := range tc.permissions {
					newTrafficPerms := sprawltest.MustSetResourceData(t, &pbresource.Resource{
						Id: &pbresource.ID{
							Type:    pbauth.TrafficPermissionsType,
							Name:    "static-server-perms" + strconv.Itoa(i) + "-" + st.Namespace + "-" + st.Partition,
							Tenancy: dt,
						},
					}, newTrafficPermissions(p, st))
					mustWriteTestResource(t, clientV2, newTrafficPerms)
					initialTrafficPerms = append(initialTrafficPerms, newTrafficPerms)
				}
			}
		}
		t.Log(initialTrafficPerms)
		// Wait for the resource updates to go through and Envoy to be ready
		time.Sleep(1 * time.Second)
		// Check the default server workload envoy config for RBAC filters matching testcase criteria
		serverWorkload := cluster.WorkloadsByID(topology.ID{
			Partition: "default",
			Namespace: "default",
			Name:      "static-server",
		})
		asserter.AssertEnvoyHTTPrbacFiltersContainIntentions(t, serverWorkload[0])
		// Check relationships
		for _, ship := range ships {
			t.Run("case: "+testName+":"+ship.Destination.PortName+":("+ship.Caller.ID.Partition+"/"+ship.Caller.ID.Namespace+
				")("+ship.Destination.ID.Partition+"/"+ship.Destination.ID.Namespace+")", func(t *testing.T) {
				var (
					wrk  = ship.Caller
					dest = ship.Destination
				)
				for _, res := range tc.result {
					if res.port != "" && res.port != ship.Destination.PortName {
						continue
					}
					dest.ID.Name = "static-server"
					destClusterPrefix := clusterPrefix(dest.PortName, dest.ID, dest.Cluster)
					asserter.DestinationEndpointStatus(t, wrk, destClusterPrefix+".", "HEALTHY", len(tenancies))
					status := http.StatusForbidden
					if res.fail == false {
						status = http.StatusOK
					}
					t.Log("Test request:"+res.path, res.headers, status)
					asserter.FortioFetch2ServiceStatusCodes(t, wrk, dest, res.path, res.headers, []int{status})
				}
			})
		}
	}
}

func mustWriteTestResource(t *testing.T, client pbresource.ResourceServiceClient, res *pbresource.Resource) {
	retryer := &retry.Timer{Timeout: time.Minute, Wait: time.Second}
	rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	retry.RunWith(retryer, t, func(r *retry.R) {
		readRsp, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: rsp.Resource.Id})
		require.NoError(r, err, "error reading %s", rsp.Resource.Id.Name)
		require.NotNil(r, readRsp)
	})

}

func mustDeleteTestResources(t *testing.T, client pbresource.ResourceServiceClient, resources []*pbresource.Resource) {
	if len(resources) == 0 {
		return
	}
	retryer := &retry.Timer{Timeout: time.Minute, Wait: time.Second}
	for _, res := range resources {
		retry.RunWith(retryer, t, func(r *retry.R) {
			_, err := client.Delete(context.Background(), &pbresource.DeleteRequest{Id: res.Id})
			if status.Code(err) == codes.NotFound {
				return
			}
			if err != nil && status.Code(err) != codes.Aborted {
				r.Stop(fmt.Errorf("failed to delete the resource: %w", err))
				return
			}
			require.NoError(r, err)
		})
	}
}

type testL7TrafficPermissionsCreator struct {
	tenancies []*pbresource.Tenancy
}

func (c testL7TrafficPermissionsCreator) NewConfig(t *testing.T) *topology.Config {
	const clusterName = "dc1"

	servers := topoutil.NewTopologyServerSet(clusterName+"-server", 1, []string{clusterName, "wan"}, nil)

	cluster := &topology.Cluster{
		Enterprise: utils.IsEnterprise(),
		Name:       clusterName,
		Nodes:      servers,
	}

	lastNode := 0
	nodeName := func() string {
		lastNode++
		return fmt.Sprintf("%s-box%d", clusterName, lastNode)
	}

	for _, st := range c.tenancies {
		for _, dt := range c.tenancies {
			c.topologyConfigAddNodes(cluster, nodeName, st, dt)

		}
	}

	return &topology.Config{
		Images: utils.TargetImages(),
		Networks: []*topology.Network{
			{Name: clusterName},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			cluster,
		},
	}
}

func (c testL7TrafficPermissionsCreator) topologyConfigAddNodes(
	cluster *topology.Cluster,
	nodeName func() string,
	sourceTenancy *pbresource.Tenancy,
	destinationTenancy *pbresource.Tenancy,
) {
	clusterName := cluster.Name

	newID := func(name string, tenancy *pbresource.Tenancy) topology.ID {
		return topology.ID{
			Partition: tenancy.Partition,
			Namespace: tenancy.Namespace,
			Name:      name,
		}
	}

	serverNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: destinationTenancy.Partition,
		Name:      nodeName(),
		Workloads: []*topology.Workload{
			topoutil.NewFortioWorkloadWithDefaults(
				clusterName,
				newID("static-server", destinationTenancy),
				topology.NodeVersionV2,
				nil,
			),
		},
	}

	clientNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: sourceTenancy.Partition,
		Name:      nodeName(),
		Workloads: []*topology.Workload{
			topoutil.NewFortioWorkloadWithDefaults(
				clusterName,
				newID("static-client", sourceTenancy),
				topology.NodeVersionV2,
				func(wrk *topology.Workload) {
					wrk.Destinations = append(wrk.Destinations, &topology.Destination{
						ID:           newID("static-server", destinationTenancy),
						PortName:     "http",
						LocalAddress: "0.0.0.0", // needed for an assertion
						LocalPort:    5000,
					},
						&topology.Destination{
							ID:           newID("static-server", destinationTenancy),
							PortName:     "http2",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5001,
						},
					)
					wrk.WorkloadIdentity = "static-client"
				},
			),
		},
	}

	cluster.Nodes = append(cluster.Nodes,
		clientNode,
		serverNode,
	)
}
