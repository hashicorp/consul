ARG ALPINE_VERSION=3.7
FROM alpine:${ALPINE_VERSION}

ARG NODEJS_VERSION=8.9.3-r1
ARG MAKE_VERSION=4.2.1-r0
ARG YARN_VERSION=1.7.0

RUN apk update && \
    apk add nodejs=${NODEJS_VERSION} nodejs-npm=${NODEJS_VERSION} make=${MAKE_VERSION} rsync && \
    npm config set unsafe-perm true && \
    npm install --global yarn@${YARN_VERSION} && \
    mkdir /consul-src

WORKDIR /consul-src
CMD make
