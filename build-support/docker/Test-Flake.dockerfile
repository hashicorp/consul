FROM travisci/ci-garnet:packer-1512502276-986baf0

ENV GOLANG_VERSION 1.12.8

RUN mkdir -p /home/travis/go && chown -R travis /home/travis/go

ENV GOPATH /home/travis/go

USER travis

COPY flake.sh /usr/local/bin/flake.sh

ENTRYPOINT [ "bash", "flake.sh" ]