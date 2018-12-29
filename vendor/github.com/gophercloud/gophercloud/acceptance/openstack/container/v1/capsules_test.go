package v1

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/openstack/container/v1/capsules"
	"github.com/gophercloud/gophercloud/pagination"
	th "github.com/gophercloud/gophercloud/testhelper"
)

func TestCapsule(t *testing.T) {
	client, err := clients.NewContainerV1Client()
	if err != nil {
		t.Fatalf("Unable to create an container v1 client: %v", err)
	}
	th.AssertNoErr(t, err)
	template := new(capsules.Template)
	template.Bin = []byte(`{
		"capsuleVersion": "beta",
		"kind": "capsule",
		"metadata": {
			"labels": {
				"app": "web",
				"app1": "web1"
			},
			"name": "template"
		},
		"spec": {
			"restartPolicy": "Always",
			"containers": [
				{
					"command": [
						"sleep",
						"1000000"
					],
					"env": {
						"ENV1": "/usr/local/bin",
						"ENV2": "/usr/bin"
					},
					"image": "ubuntu",
					"imagePullPolicy": "ifnotpresent",
					"ports": [
						{
							"containerPort": 80,
							"hostPort": 80,
							"name": "nginx-port",
							"protocol": "TCP"
						}
					],
					"resources": {
						"requests": {
							"cpu": 1,
							"memory": 1024
						}
					},
					"workDir": "/root"
				}
			]
		}
	}`)
	createOpts := capsules.CreateOpts{
		TemplateOpts: template,
	}
	capsule, err := capsules.Create(client, createOpts).Extract()
	th.AssertNoErr(t, err)
	err = WaitForCapsuleStatus(client, capsule, "Running")
	th.AssertNoErr(t, err)
	pager := capsules.List(client, nil)
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		CapsuleList, err := capsules.ExtractCapsules(page)
		th.AssertNoErr(t, err)

		for _, m := range CapsuleList {
			capsuleUUID := m.UUID
			capsule, err := capsules.Get(client, capsuleUUID).Extract()

			th.AssertNoErr(t, err)
			th.AssertEquals(t, capsule.MetaName, "template")

			err = capsules.Delete(client, capsuleUUID).ExtractErr()
			th.AssertNoErr(t, err)

		}
		return true, nil
	})
	th.AssertNoErr(t, err)
}
