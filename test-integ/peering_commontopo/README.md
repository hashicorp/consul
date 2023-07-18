# CONSUL PEERING COMMON TOPOLOGY TESTS

These peering tests all use a `commonTopo` (read: "common topology") to enable sharing a deployment of a Consul. Sharing a deployment of Consul cuts down on setup time.

To run these tests, you will need to have docker installed. Next, make sure that you have all the required consul containers built:

```
make test-compat-integ-setup
```

## Non-Shared CommonTopo Tests

The tests in question are designed in a manner that modifies the topology. As a result, it is noy possible to share the testing environment across these tests.

## Shared CommonTopo Tests

The tests in question are designed in a manner that does not modify the topology. As a result, it is possible to share the testing environment across these tests.

To run all consul peering tests with shared and non shared topology, run the following command:

```
cd /path/to/peering_commontopo
go test -timeout=10m  -commontopo -v . 
```

To run all peering tests with shared topology, run the following command:

```
cd /path/to/peering_commontopo
go test -timeout=10m -run ^'TestSuitesOnSharedTopo' -commontopo -v . 
```

To run individual tests:

```
cd /path/to/peering_commontopo
go test -timeout=10m -run ^'TestAC1Basic' -v .    
```

## Local Development and Testing

All the methods in the `commonTopoSuite` interface must be implemented.

- `testName()` prepends the test suite name to each test in the test suite.
- `setup()` phase must ensure that any resources added to the topology cannot interfere with other tests. Principally by prefixing.
- `test()` phase must be "passive" and not mutate the topology in any way that would interfere with other tests.

Common topology peering tests are defined in the [test-integ/peering_commontopo/](/test-integ/peering_commontopo/) subdirectory and new peering integration tests should always be added to this location. Adding integration tests that does not modify the topology should always start by invoking

```go
setupAndRunTestSuite(t, ac1BasicSuites, true, true)
```

else

```go
setupAndRunTestSuite(t, ac1BasicSuites, false, false)
```

Some of these tests *do* mutate in their `test()` phase, and while they use `commonTopo` for the purpose of code sharing, they are not included in the "shared topo" tests in `sharedtopology_test.go`.
