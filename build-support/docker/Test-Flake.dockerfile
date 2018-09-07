FROM travisci/ci-garnet:packer-1512502276-986baf0

ENV GOLANG_VERSION 1.10.3

RUN mkdir -p /home/travis/go && chown -R travis /home/travis/go

ENV GOPATH /home/travis/go

USER travis

COPY test.sh /usr/local/bin/test.sh

ENTRYPOINT [ "bash", "test.sh" ]