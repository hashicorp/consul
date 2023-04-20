# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

FROM consul:local
EXPOSE 4000

COPY --from=golang:1.20-alpine /usr/local/go/ /usr/local/go/

ENV PATH="/usr/local/go/bin:${PATH}"

RUN CGO_ENABLED=0 go install -ldflags "-s -w -extldflags '-static'" github.com/go-delve/delve/cmd/dlv@latest

CMD [ "/root/go/bin/dlv", "exec", "/bin/consul", "--listen=:4000", "--headless=true", "", "--accept-multiclient", "--continue", "--api-version=2", "--", "agent", "--advertise=0.0.0.0"]