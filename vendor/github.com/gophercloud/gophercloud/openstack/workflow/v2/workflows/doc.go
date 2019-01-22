/*
Package workflows provides interaction with the workflows API in the OpenStack Mistral service.

Workflow represents a process that can be described in a various number of ways and that can do some job interesting to the end user.
Each workflow consists of tasks (at least one) describing what exact steps should be made during workflow execution.

Example to list workflows

	listOpts := workflows.ListOpts{
		Namespace: "some-namespace",
	}

	allPages, err := workflows.List(mistralClient, listOpts).AllPages()
	if err != nil {
		panic(err)
	}

	allWorkflows, err := workflows.ExtractWorkflows(allPages)
	if err != nil {
		panic(err)
	}

	for _, workflow := range allWorkflows {
		fmt.Printf("%+v\n", workflow)
	}

Example to create a workflow

	workflowDefinition := `---
version: '2.0'

create_vm:
description: Simple workflow example
type: direct

input:
	- vm_name
	- image_ref
	- flavor_ref
output:
	vm_id: <% $.vm_id %>

tasks:
	create_server:
	action: nova.servers_create name=<% $.vm_name %> image=<% $.image_ref %> flavor=<% $.flavor_ref %>
	publish:
		vm_id: <% task(create_server).result.id %>
	on-success:
		- wait_for_instance

	wait_for_instance:
	action: nova.servers_find id=<% $.vm_id %> status='ACTIVE'
	retry:
		delay: 5
		count: 15`

	createOpts := &workflows.CreateOpts{
		Definition: strings.NewReader(workflowDefinition),
		Namespace: "some-namespace",
	}

	execution, err := workflows.Create(fake.ServiceClient(), opts).Extract()
	if err != nil {
		panic(err)
	}
*/
package workflows
