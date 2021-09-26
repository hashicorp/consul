ARG ALPINE_VERSION=3.13
FROM alpine:${ALPINE_VERSION}

ARG NODEJS_VERSION=14.17.6-r0
ARG MAKE_VERSION=4.3-r0
ARG YARN_VERSION=1.22.10

RUN apk update && \
    apk add nodejs=${NODEJS_VERSION} nodejs-npm=${NODEJS_VERSION} make=${MAKE_VERSION} && \
    npm config set unsafe-perm true && \
    npm install --global yarn@${YARN_VERSION} && \
    mkdir /consul-src

WORKDIR /consul-src
CMD make dist-docker
