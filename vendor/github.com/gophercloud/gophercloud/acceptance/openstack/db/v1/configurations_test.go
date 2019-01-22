// +build acceptance db

package v1

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/db/v1/configurations"
)

func TestConfigurationsCRUD(t *testing.T) {
	client, err := clients.NewDBV1Client()
	if err != nil {
		t.Fatalf("Unable to create a DB client: %v", err)
	}

	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		t.Fatalf("Unable to get environment settings")
	}

	createOpts := &configurations.CreateOpts{
		Name:        "test",
		Description: "description",
	}

	datastore := configurations.DatastoreOpts{
		Type:    choices.DBDatastoreType,
		Version: choices.DBDatastoreVersion,
	}
	createOpts.Datastore = &datastore

	values := make(map[string]interface{})
	values["collation_server"] = "latin1_swedish_ci"
	createOpts.Values = values

	cgroup, err := configurations.Create(client, createOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to create configuration: %v", err)
	}

	err = configurations.Delete(client, cgroup.ID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete configuration: %v", err)
	}

	tools.PrintResource(t, cgroup)
}
