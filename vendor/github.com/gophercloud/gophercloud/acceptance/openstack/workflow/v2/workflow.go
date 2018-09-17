package v2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/workflow/v2/workflows"
	th "github.com/gophercloud/gophercloud/testhelper"
)

// GetEchoWorkflowDefinition returns a simple workflow definition that does nothing except a simple "echo" command.
func GetEchoWorkflowDefinition(workflowName string) string {
	return fmt.Sprintf(`---
version: '2.0'

%s:
  description: Simple workflow example
  type: direct

  tasks:
    test:
      action: std.echo output="Hello World!"`, workflowName)
}

// CreateWorkflow creates a workflow on Mistral API.
// The created workflow is a dummy workflow that performs a simple echo.
func CreateWorkflow(t *testing.T, client *gophercloud.ServiceClient) (*workflows.Workflow, error) {
	workflowName := tools.RandomString("workflow_echo_", 5)

	definition := GetEchoWorkflowDefinition(workflowName)

	t.Logf("Attempting to create workflow: %s", workflowName)

	opts := &workflows.CreateOpts{
		Namespace:  "some-namespace",
		Scope:      "private",
		Definition: strings.NewReader(definition),
	}
	workflowList, err := workflows.Create(client, opts).Extract()
	if err != nil {
		return nil, err
	}
	th.AssertEquals(t, 1, len(workflowList))

	workflow := workflowList[0]

	t.Logf("Workflow created: %s", workflowName)

	th.AssertEquals(t, workflowName, workflow.Name)

	return &workflow, nil
}
