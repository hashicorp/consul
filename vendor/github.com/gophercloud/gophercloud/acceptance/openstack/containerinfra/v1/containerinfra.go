package v1

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/containerinfra/v1/clusters"
	"github.com/gophercloud/gophercloud/openstack/containerinfra/v1/clustertemplates"
	th "github.com/gophercloud/gophercloud/testhelper"
)

// CreateClusterTemplate will create a random cluster tempalte. An error will be returned if the
// cluster-template could not be created.
func CreateClusterTemplate(t *testing.T, client *gophercloud.ServiceClient) (*clustertemplates.ClusterTemplate, error) {
	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		return nil, err
	}

	name := tools.RandomString("TESTACC-", 8)
	t.Logf("Attempting to create cluster template: %s", name)

	boolFalse := false
	createOpts := clustertemplates.CreateOpts{
		COE:                 "swarm",
		DNSNameServer:       "8.8.8.8",
		DockerStorageDriver: "devicemapper",
		ExternalNetworkID:   choices.ExternalNetworkID,
		FlavorID:            choices.FlavorID,
		FloatingIPEnabled:   &boolFalse,
		ImageID:             choices.MagnumImageID,
		MasterFlavorID:      choices.FlavorID,
		MasterLBEnabled:     &boolFalse,
		Name:                name,
		Public:              &boolFalse,
		RegistryEnabled:     &boolFalse,
		ServerType:          "vm",
	}

	res := clustertemplates.Create(client, createOpts)
	if res.Err != nil {
		return nil, res.Err
	}

	requestID := res.Header.Get("X-OpenStack-Request-Id")
	th.AssertEquals(t, true, requestID != "")

	t.Logf("Cluster Template %s request ID: %s", name, requestID)

	clusterTemplate, err := res.Extract()
	if err != nil {
		return nil, err
	}

	t.Logf("Successfully created cluster template: %s", clusterTemplate.Name)

	tools.PrintResource(t, clusterTemplate)
	tools.PrintResource(t, clusterTemplate.CreatedAt)

	th.AssertEquals(t, name, clusterTemplate.Name)
	th.AssertEquals(t, choices.ExternalNetworkID, clusterTemplate.ExternalNetworkID)
	th.AssertEquals(t, choices.MagnumImageID, clusterTemplate.ImageID)

	return clusterTemplate, nil
}

// DeleteClusterTemplate will delete a given cluster-template. A fatal error will occur if the
// cluster-template could not be deleted. This works best as a deferred function.
func DeleteClusterTemplate(t *testing.T, client *gophercloud.ServiceClient, id string) {
	t.Logf("Attempting to delete cluster-template: %s", id)

	err := clustertemplates.Delete(client, id).ExtractErr()
	if err != nil {
		t.Fatalf("Error deleting cluster-template %s: %s:", id, err)
	}

	t.Logf("Successfully deleted cluster-template: %s", id)

	return
}

// CreateCluster will create a random cluster. An error will be returned if the
// cluster could not be created.
func CreateCluster(t *testing.T, client *gophercloud.ServiceClient, clusterTemplateID string) (string, error) {
	clusterName := tools.RandomString("TESTACC-", 8)
	t.Logf("Attempting to create cluster: %s using template %s", clusterName, clusterTemplateID)

	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		return "", err
	}

	masterCount := 1
	nodeCount := 1
	createTimeout := 100
	createOpts := clusters.CreateOpts{
		ClusterTemplateID: clusterTemplateID,
		CreateTimeout:     &createTimeout,
		FlavorID:          choices.FlavorID,
		Keypair:           choices.MagnumKeypair,
		Labels:            map[string]string{},
		MasterCount:       &masterCount,
		MasterFlavorID:    choices.FlavorID,
		Name:              clusterName,
		NodeCount:         &nodeCount,
	}

	createResult := clusters.Create(client, createOpts)
	th.AssertNoErr(t, createResult.Err)
	if len(createResult.Header["X-Openstack-Request-Id"]) > 0 {
		t.Logf("Cluster Create Request ID: %s", createResult.Header["X-Openstack-Request-Id"][0])
	}

	clusterID, err := createResult.Extract()
	if err != nil {
		return "", err
	}

	t.Logf("Cluster created: %+v", clusterID)

	err = WaitForCluster(client, clusterID, "CREATE_COMPLETE")
	if err != nil {
		return clusterID, err
	}

	t.Logf("Successfully created cluster: %s id: %s", clusterName, clusterID)
	return clusterID, nil
}

func WaitForCluster(client *gophercloud.ServiceClient, clusterID string, status string) error {
	return tools.WaitFor(func() (bool, error) {
		cluster, err := clusters.Get(client, clusterID).Extract()
		if err != nil {
			if _, ok := err.(gophercloud.ErrDefault404); ok && status == "DELETE_COMPLETE" {
				return true, nil
			}

			return false, err
		}

		if cluster.Status == status {
			return true, nil
		}

		if strings.Contains(cluster.Status, "FAILED") {
			return false, fmt.Errorf("Cluster %s FAILED. Status=%s StatusReason=%s", clusterID, cluster.Status, cluster.StatusReason)
		}

		return false, nil
	})
}
