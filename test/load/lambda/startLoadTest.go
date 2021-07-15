package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
)

// Event defines all of our input and configuration
type Event struct {
	Deployment string `json:"deployment"`
	// Version and SHA are optional. If both are missing, default to latest version
	Version string `json:"version",omitempty`
	SHA     string `json:"sha",omitempty`
	// DeploymentID is required when tearing down an existing deployment
	DeploymentID string `json:deployment_id,omitempty`
}

// FSM contains the current state of the build and enforces state transitions to only occur between valid states.
type FSM struct{ state state }
type state string

// Declare all of our FSM states as const types
var (
	Start     state = "start"
	Build     state = "build"
	Provision state = "provision"
	Run       state = "run"
	Teardown  state = "teardown"
	Done      state = "done"
)

func newFSM() FSM {
	return FSM{state: Build}
}

// advance currently steps down the list of states, we will add support for more valid state transitions as needed.
func (m *FSM) advance(next state) (state, error) {
	switch state := m.state; state {
	// TODO PR deployments will need to support teardowns of running deploys. Add support for transition from New to Teardown
	case Start:
		m.state = Build
		return m.state, nil
	case Build:
		s.state = Provision
		return m.state, nil
	case Provision:
		s.state = Run
		return m.state, nil

	// TODO PR deployment creation will need to support teardowns of running deploys. Add support for transition from Run to Done
	case Run:
		s.state = Teardown
		return m.state, nil

	case Teardown:
		s.state = Done
		return m.state, nil

	case Done:
		return m.state, fmt.Error("unable to advance FSM, deployment is done")
	}
}

func handleLoadTest(ctx context.Context, event Event) (string, error) {
	// Start a fresh FSM
	fsm := newFSM()

	// TODO Build the package images from Consul binary taking version or SHA - store the AMI ID
	state, err := fsm.advance(Build)
	// TODO Move err string generation to fsm.advance()
	if state != Build {
		return nil, fmt.Errorf("attempted to perform build while state is %s", state)
	}

	// TODO Provision all the server by running `terraform apply` with the AMI ID
	// TODO Generate a deployment_id that will allow us to delete long-running deployments e.g. pull-request deploys
	state, err := fsm.advance(Provision)
	if state != Provision {
		return nil, fmt.Errorf("attempted to perform provision while state is %s", state)
	}

	// TODO wait for tests to run
	state, err := fsm.advance(Run)
	if state != Run {
		return nil, fmt.Errorf("attempted to perform run while state is %s", state)
	}

	// TODO Teardown the terraform cluster
	state, err := fsm.advance(Teardown)
	if state != Teardown {
		return nil, fmt.Errorf("attempted to perform teardown while state is %s", state)
	}

	// TODO Finish
	state, err := fsm.advance(Done)
	if state != Done {
		return nil, fmt.Errorf("attempted to set state to done while state is %s", state)
	}

	return fmt.Sprintf("Deployment is %s, %s success", state, event.Deployment), nil
}

func handlePullRequest(ctx context.Context, event Event) (string, error) {
	// TODO Fresh PR deploy: Start -> Build -> Provision -> Run -> Done
	// TODO Delete PR deploy: If deployment_id != nil { New -> Teardown -> Done }
	return "", fmt.Errorf("cannot deploy pull-request, not yet implemented")
}

func HandleRequest(ctx context.Context, event Event) (string, error) {
	switch deployment := event.Deployment; deployment {
	case "load-test":
		return handleLoadTest(ctx, event)
	case "pull-request":
		return handlePullRequest(ctx, event)
	default:
		return "", fmt.Errorf("unknown deployment: %s", deployment)
	}
}

func main() {
	lambda.Start(HandleRequest)
}
