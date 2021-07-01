ARG ALPINE_VERSION=3.11
FROM alpine:${ALPINE_VERSION}

ARG NODEJS_VERSION=12.22.1-r0
ARG MAKE_VERSION=4.2.1-r2
ARG YARN_VERSION=1.22.5

RUN apk update && \
    apk add nodejs=${NODEJS_VERSION} nodejs-npm=${NODEJS_VERSION} make=${MAKE_VERSION} && \
    npm config set unsafe-perm true && \
    npm install --global yarn@${YARN_VERSION} && \
    mkdir /consul-src

WORKDIR /consul-src
CMD make dist-docker
