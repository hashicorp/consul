# Controller Testing

For every controller we want to enable 3 types of testing.

1. Unit Tests - These should live alongside the controller and utilize mocks and the controller.TestController. Where possible split out controller functionality so that other functions can be independently tested.
2. Lightweight integration tests - These should live in an internal/<api group>/<api group>test package. These tests utilize the in-memory resource service and the standard controller manager. There are two types of tests that should be created.
	* Lifecycle Integration Tests - These go step by step to modify resources and check what the controller did. They are meant to go through the lifecycle of resources and how they are reconciled. Verifications are typically intermingled with resource updates.
	* One-Shot Integration Tests - These tests publish a bunch of resources and then perform all the verifications. These mainly are focused on the controller eventually converging given all the resources thrown at it and aren't as concerned with any intermediate states resources go through.
3. Container based integration tests - These tests live along with our other container based integration tests. They utilize a full multi-node cluster (and sometimes client agents). There are 3 types of tests that can be created here:
   * Lifecycle Integration Tests - These are the same as for the lighweight integration tests.
	* One-shot IntegrationTests - These are the same as for the lightweight integration tests.
	* Upgrade Tests - These are a special form of One-shot Integration tests where the cluster is brought up with some original version, data is pushed in, an upgrade is done and then we verify the consistency of the data post-upgrade.
	
	
Between the lightweight and container based integration tests there is a lot of duplication in what is being tested. For this reason these integration test bodies should be defined as exported functions within the apigroups test package. The container based tests can then import those packages and invoke the same functionality with minimal overhead.

For one-shot integration tests, functions to do the resource publishing should be split from functions to perform the verifications. This allows upgrade tests to publish the resources once pre-upgrade and then validate that their correctness post-upgrade without requiring rewriting them.

Sometimes it may also be a good idea to export functions in the test packages for running a specific controllers integration tests. This is a good idea when the controller will use a different version of a dependency in Consul Enterprise to allow for the enterprise implementations package to invoke the integration tests after setting up the controller with its injected dependency.

## Unit Test Template

These tests live alongside controller source.

```go
package foo

import (
	"testing"
	
	"github.com/stretchr/testif/mock"
	"github.com/stretchr/testif/require"
	"github.com/stretchr/testif/suite"
)

func TestReconcile(t *testing.T) {
	rtest.RunWithTenancies(func(tenancy *pbresource.Tenancy) {
		suite.Run(t, &reconcileSuite{tenancy: tenancy})
	})
}

type reconcileSuite struct {
	suite.Suite
	
	tenancy *pbresource.Tenancy
	
	ctx context.Context
	ctl *controller.TestController
	client *rtest.Client
	
	// Mock objects needed for testing
}

func (suite *reconcileSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())

	// Alternatively it is sometimes useful to use a mock resource service. For that
	// you can use github.com/hashicorp/consul/grpcmocks.NewResourceServiceClient
	// to create the client. 
	client := svctest.NewResourceServiceBuilder().
		// register this API groups types. Also register any other
		// types this controller depends on.
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancy).
		Run(suite.T())
		
	// Build any mock objects or other dependencies of the controller here.

	// Build the TestController
	suite.ctl = controller.NewTestController(Controller(), client)
	suite.client = rtest.NewClient(suite.ctl.Runtime().Client)
}

// Implement tests on the suite as needed.
func (suite *reconcileSuite) TestSomething() {
	// Setup Mock expectations
	
	// Push resources into the resource service as needed.
	
	// Issue the Reconcile call
	suite.ctl.Reconcile(suite.ctx, controller.Request{})
}
```

## Integration Testing Templates

These tests should live in internal/<api group>/<api group>test. For these examples, assume the API group under test is named `foo` and the latest API group version is v2.

### `run_test.go`

This file is how `go test` knows to execute the tests. These integration tests should
be executed against an in-memory resource service with the standard controller manager.

```go
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package footest

import (
	"testing"

	"github.com/hashicorp/consul/internal/foo"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/resource/reaper"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	// This makes the CLI options available to control timing delays of requests. The
	// randomized timings helps to build confidence that regardless of resources writes
	// occurring in quick succession, the controller under test will eventually converge
	// on its steady state.
	clientOpts = rtest.ConfigureTestCLIFlags()
)

func runInMemResourceServiceAndControllers(t *testing.T) pbresource.ResourceServiceClient {
	t.Helper()

	return controllertest.NewControllerTestBuilder().
		// Register your types for the API group and any others that these tests will depend on
		WithResourceRegisterFns(types.Register).
		WithControllerRegisterFns(
			reaper.RegisterControllers,
			foo.RegisterControllers,
		).Run(t)
}

// The basic integration test should operate mostly in a one-shot manner where resources
// are published and then verifications are performed. 
func TestControllers_Integration(t *testing.T) {
	client := runInMemResourceServiceAndControllers(t)
	RunFooV2IntegrationTest(t, client, clientOpts.ClientOptions(t)...)
}

// The lifecycle integration test is typically more complex and deals with changing
// some values over time to cause the controllers to do something differently.
func TestControllers_Lifecycle(t *testing.T) {
	client := runInMemResourceServiceAndControllers(t)
	RunFooV2LifecycleTest(t, client, clientOpts.ClientOptions(t)...)
}

```

### `test_integration_v2.go`


```go
package footest

import (
	"embed"
	"fmt"
	"testing"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	//go:embed integration_test_data
	testData embed.FS
)

// Execute the full integration test
func RunFooV2IntegrationTest(t *testing.T, client pbresource.ResourceServiceClient, opts ...rtest.ClientOption) {
	t.Helper
	
	PublishFooV2IntegrationTestData(t, client, opts...)
	VerifyFooV2IntegrationTestResults(t, client)
}

// PublishFooV2IntegrationTestData publishes all the data that needs to exist in the resource service
// for the controllers to converge on the desired state.
func PublishFooV2IntegrationTestData(t *testing.T, client pbresource.ResourceServiceClient, opts ...rtest.ClientOption) {
	t.Helper()

	c := rtest.NewClient(client, opts...)

	// Publishing resources manually is an option but alternatively you can store the resources on disk
	// and use go:embed declarations to embed the whole test data filesystem into the test binary.
	resources := rtest.ParseResourcesFromFilesystem(t, testData, "integration_test_data/v2")
	c.PublishResources(t, resources)
}

func VerifyFooV2IntegrationTestResults(t *testing.T, client pbresource.ResourceServiceClient) {
	t.Helper()
	
	c := rtest.NewClient(client)
	
	// Perform verifications here. All verifications should be retryable except in very exceptional circumstances.
	// This could be in a retry.Run block or could be retryed by using one of the WaitFor* methods on the rtest.Client.
	// Having them be retryable will prevent flakes especially when the verifications are run in the context of
	// a multi-server cluster where a raft follower hasn't yet observed some change.
}
```

### `test_lifecycle_v2.go`

```go
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package footest

import (
	"testing"

	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RunFooV2LifecycleIntegrationTest(t *testing.T, client pbresource.ResourceServiceClient, opts ...rtest.ClientOption) {
	t.Helper()
	
	// execute tests.
}
```
