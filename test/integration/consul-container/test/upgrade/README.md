# Consul Upgrade Integration tests
## Local run
- run `make dev-docker`
- run the tests, e.g., `go test -run ^TestBasicConnectService$ ./test/basic -v`

To specify targets and latest image pass `target-version` and `latest-version` to the tests. By default, it uses the `consul` docker image with respectively `local` and `latest` tags.

To use dev consul image, pass `target-image` and `target-version`, `-target-image hashicorppreview/consul -target-version 1.14-dev`.

By default, all container's logs are written to either `stdout`, or `stderr`; this makes it hard to debug, when the test case creates many
containers. To disable following container logs, run the test with `-follow-log false`.
