# CONSUL PEERING COMMON TOPOLOGY TESTS

These peering tests all use a `commonTopo` (read: "common topology") to enable sharing a deployment of a Consul. Sharing a deployment of Consul cuts down on setup time.

To run these tests, you will need to have docker installed. Next, make sure that you have all the required consul containers built:

```
make test-compat-integ-setup
```

## Non-Shared CommonTopo Tests

The tests in question are designed in a manner that modifies the topology. As a result, it is not possible to share the testing environment across these tests.

## Shared CommonTopo Tests

The tests in question are designed in a manner that does not modify the topology in any way that would interfere with other tests. As a result, it is possible to share the testing environment across these tests.

To run all consul peering tests with no shared topology, run the following command:

```
cd /path/to/peering_commontopo
go test -timeout=10m -v -no-share-topo . 
```

To run all peering tests with shared topology only:

```
cd /path/to/peering_commontopo
go test -timeout=10m -run '^TestSuitesOnSharedTopo' -v . 
```

To run individual peering topology tests:

```
cd /path/to/peering_commontopo
go test -timeout=10m -run '^TestSuiteExample' -v -no-share-topo .    
```

## Local Development and Testing

If writing tests for peering with no shared topology, this recommendation does not apply. The following methods below not necessarily need to be implmented. For shared topology tests, all the methods in the `sharedTopoSuite` interface must be implemented.

- `testName()` prepends the test suite name to each test in the test suite.
- `setup()` phase must ensure that any resources added to the topology cannot interfere with other tests. Principally by prefixing.
- `test()` phase must be "passive" and not mutate the topology in any way that would interfere with other tests.

Common topology peering tests are defined in the [test-integ/peering_commontopo/](/test-integ/peering_commontopo/) directory and new peering integration tests should always be added to this location. Adding integration tests that does not modify the topology should always start by invoking

```go
runShareableSuites(t, testSuiteExample)
```

else

```go
func TestSuiteExample(t *testing.T) {
 ct := NewCommonTopo(t)
 s := &testSuiteExample{}
 s.setup(t, ct)
 ct.Launch(t)
 s.test(t, ct)
}
```

Some of these tests *do* mutate in their `test()` phase, and while they use `commonTopo` for the purpose of code sharing, they are not included in the "shared topo" tests in `sharedtopology_test.go`.
