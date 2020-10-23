FROM fortio/fortio AS fortio

FROM bats/bats:latest

RUN apk add curl
RUN apk add openssl
RUN apk add jq
COPY --from=fortio /usr/bin/fortio /usr/sbin/fortio
