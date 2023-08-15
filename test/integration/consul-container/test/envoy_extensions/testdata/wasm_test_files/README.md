# Building WASM test files

We have seen some issues with building the wasm test files on the build runners for the integration test. Currently,
the theory is that there may be some differences in the clang toolchain on different runners which cause panics in
tinygo if the job is scheduled on particular runners but not others.

In order to get around this, we are just building the wasm test file and checking it into the repo.

To build the wasm test file, 

```bash
~/consul/test/integration/consul-container/test/envoy_extensions/testdata/wasm_test_files
> docker run -v ./:/wasm --rm $(docker build -q .)
```