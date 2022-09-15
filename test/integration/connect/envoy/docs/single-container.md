# Single Container Test Architecture

## Index

- [About](#about)
- [Docker Image Components](#docker-image-components)
  - Main Components:
    - [Bats](#bats)
    - [Fortio](#fortio)
    - [Jaegertracing](#jaegertracing)
    - [Openzipkin](#openzipkin)
    - [Socat](#socat)
  - Additional tools:
    - [Git Bash](#git-bash)
    - [JQ](#jq)
    - [Netcat](#netcat)
    - [Openssl](#openssl)

## About

The purpose of this document is to describe how the Single Container test architecture is composed.

## Docker Image Components

The Docker image used for the Consul - Envoy integration tests has several components needed to run those tests.

- Main Components:
  - [Bats](#bats)
  - [Fortio](#fortio)
  - [Jaegertracing](#jaegertracing)
  - [Openzipkin](#openzipkin)
  - [Socat](#socat)
- Additional tools:
  - [Git Bash](#git-bash)
  - [JQ](#jq)
  - [Netcat](#netcat)
  - [Openssl](#openssl)

### Bats

BATS stands for Bash Automated Testing System and is the one in charge of executing the tests.

### Fortio

Fortio is a microservices (http, grpc) load testing library, command line tool, advanced echo server, and web UI. It is used to run the services registered into Consul during the integration tests.

### Jaegertracing

Jaeger is open source software for tracing transactions between distributed services. It's used for monitoring and troubleshooting complex microservices environments. It is used along with Openzipkin in some test cases.

### Openzipkin

Zipkin is also a tracing software.

### Socat

Socat is a command line based utility that establishes two bidirectional byte streams and transfers data between them. On this integration tests it is used to redirect Envoy's stats. There is no official Windows version. We are using this unofficial release available [here](https://github.com/tech128/socat-1.7.3.0-windows).

### Git Bash

This tool is only used in Windows tests, it was added to the Docker image to be able to use some Linux commands during test execution.  

### JQ

Jq is a lightweight and flexible command-line JSON processor. It is used in several tests to modify and filter JSON outputs.

### Netcat

Netcat is a simple program that reads and writes data across networks, much the same way that cat reads and writes data to files.

### Openssl

Open SSL is an all-around cryptography library that offers open-source application of the TLS protocol. It is used to verify that the correct tls certificates are being provisioned during tests.
