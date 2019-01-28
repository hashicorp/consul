FROM ubuntu:bionic

RUN mkdir -p /consul-src/ui

RUN apt-get update -y && \
    apt-get install --no-install-recommends -y -q \
            build-essential \
            git \
            ruby \
            ruby-dev \
            zip \
            zlib1g-dev && \
    gem install bundler -v '1.17.3'

WORKDIR /consul-src/ui
CMD make dist
