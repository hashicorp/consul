# Consul Upgrade Integration tests
## Local run
- run `make dev-docker`
- run the tests.

To specify targets and latest image pass `target-version` and `latest-version` to the tests. By default, it uses the `consul` docker image with respectively `local` and `latest` tags.

To use dev consul image, pass `target-image` and `target-version`, `-target-image hashicorppreview/consul -target-version 1.14-dev`.
