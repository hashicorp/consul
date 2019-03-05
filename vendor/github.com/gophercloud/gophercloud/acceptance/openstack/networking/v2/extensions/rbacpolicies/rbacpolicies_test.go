// +build acceptance

package rbacpolicies

import (
	"os"
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	projects "github.com/gophercloud/gophercloud/acceptance/openstack/identity/v3"
	networking "github.com/gophercloud/gophercloud/acceptance/openstack/networking/v2"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/rbacpolicies"
)

func TestRBACPolicyCRUD(t *testing.T) {
	username := os.Getenv("OS_USERNAME")
	if username != "admin" {
		t.Skip("must be admin to run this test")
	}

	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	// Create a network
	network, err := networking.CreateNetwork(t, client)
	if err != nil {
		t.Fatalf("Unable to create network: %v", err)
	}
	defer networking.DeleteNetwork(t, client, network.ID)

	tools.PrintResource(t, network)

	identityClient, err := clients.NewIdentityV3Client()
	if err != nil {
		t.Fatalf("Unable to obtain an identity client: %v")
	}

	// Create a project/tenant
	project, err := projects.CreateProject(t, identityClient, nil)
	if err != nil {
		t.Fatalf("Unable to create project: %v", err)
	}
	defer projects.DeleteProject(t, identityClient, project.ID)

	tools.PrintResource(t, project)

	// Create a rbac-policy
	rbacPolicy, err := CreateRBACPolicy(t, client, project.ID, network.ID)
	if err != nil {
		t.Fatalf("Unable to create rbac-policy: %v", err)
	}
	defer DeleteRBACPolicy(t, client, rbacPolicy.ID)

	tools.PrintResource(t, rbacPolicy)

	// Create another project/tenant for rbac-update
	project2, err := projects.CreateProject(t, identityClient, nil)
	if err != nil {
		t.Fatalf("Unable to create project2: %v", err)
	}
	defer projects.DeleteProject(t, identityClient, project2.ID)

	tools.PrintResource(t, project2)

	// Update a rbac-policy
	updateOpts := rbacpolicies.UpdateOpts{
		TargetTenant: project2.ID,
	}

	_, err = rbacpolicies.Update(client, rbacPolicy.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update rbac-policy: %v", err)
	}

	// Get the rbac-policy by ID
	t.Logf("Get rbac_policy by ID")
	newrbacPolicy, err := rbacpolicies.Get(client, rbacPolicy.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to retrieve rbac policy: %v", err)
	}

	tools.PrintResource(t, newrbacPolicy)
}

func TestRBACPolicyList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	type rbacPolicy struct {
		rbacpolicies.RBACPolicy
	}

	var allRBACPolicies []rbacPolicy

	allPages, err := rbacpolicies.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list rbac policies: %v", err)
	}

	err = rbacpolicies.ExtractRBACPolicesInto(allPages, &allRBACPolicies)
	if err != nil {
		t.Fatalf("Unable to extract rbac policies: %v", err)
	}

	for _, rbacpolicy := range allRBACPolicies {
		tools.PrintResource(t, rbacpolicy)
	}
}
