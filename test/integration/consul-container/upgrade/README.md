# Consul Upgrade Integration tests
## Local run
- cd to consul root directory
- docker build -t consul:local -f build-support/docker/Consul-Dev-Build.dockerfile .
- run the tests.

To specify targets and latest image pass `target-version` and `latest-version` to the tests. By default, it uses the `consul` docker image with respectively `local` and `latest` tags.