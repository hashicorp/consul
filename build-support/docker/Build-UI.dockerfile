FROM docker.mirror.hashicorp.services/circleci/node:14-browsers

USER root

RUN mkdir /consul-src
WORKDIR /consul-src
CMD make dist-docker
