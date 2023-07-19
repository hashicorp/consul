# peering_commontopo

These peering tests all use a `commonTopo` (read: "common topology") to enable sharing a deployment of a Consul. Sharing a deployment of Consul cuts down on setup time.

This is only possible if two constraints are followed:

- `setup()` phase must ensure that any resources added to the topology cannot interfere with other tests. Principally by prefixing.
- `test()` phase must be "passive" and not mutate the topology in any way that would interfere with other tests.

Some of these tests *do* mutate in their `test()` phase, and while they use `commonTopo` for the purpose of code sharing, they are not included in the "shared topo" tests in `all_sharedtopo_test.go`.

Tests that are "shared topo" can also be run in an independent manner, gated behind the `-no-reuse-common-topo` flag. The same flag also prevents the shared topo suite from running. So `go test .` (without the flag) runs all shared topo-capable tests in *shared topo mode*, as well as shared topo-incapable tests; and `go test -no-reuse-common-topo` runs all shared topo-capable tests *individidually*, as well as the shared topo-incapable tests. Mostly this is so that when working on a single test, you don't also need to run other tests, but by default when running `go test .` the usual way, you run all tests in the fastest way.