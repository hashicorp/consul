<<<<<<< HEAD
ARG CONSUL_IMAGE_VERSION=latest
FROM consul:${CONSUL_IMAGE_VERSION}
COPY consul /bin/consul
=======
ARG CONSUL_BUILD_IMAGE
FROM ${CONSUL_BUILD_IMAGE}:latest as builder
# FROM golang:latest as builder
ARG GIT_COMMIT
ARG GIT_DIRTY
ARG GIT_DESCRIBE
# WORKDIR /go/src/github.com/hashicorp/consul
ENV CONSUL_DEV=1
ENV COLORIZE=0

# Cache modules separately from more frequently edited source files.
#
# The trick is taken from [https://medium.com/@pliutau/docker-and-go-modules-4265894f9fc#6622]
#
# We copy the modules files in first since they are less likely to change frequently
# and the population of the go mod cache will be invalidated less frequently.
COPY go.mod .
COPY go.sum .
RUN mkdir -p api sdk
COPY api/go.mod api
COPY api/go.sum api
COPY sdk/go.mod sdk
COPY sdk/go.sum sdk
RUN go mod download

# Add the rest of the code.
ADD . /consul/
RUN make dev

# PACKAGE STAGE
#
# Note that this stage is also copied in Consul-Dev.dockerfile so that CI can
# use the already-built binary and just copy it into a container so we don't
# have to wait for a whole other build just to get a version we can run in
# docker-compose integration tests.
FROM consul:latest

# Add a label which lets us easily tell if we have changes to build in the
# current tree.
ARG GIT_DESCRIBE
LABEL "com.hashicorp.consul.version"=${GIT_DESCRIBE}

COPY --from=builder /go/bin/consul /bin
>>>>>>> Basic Envoy integration test framework
