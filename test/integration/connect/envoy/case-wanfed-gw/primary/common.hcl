# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

tls {
  internal_rpc {
    ca_file                = "/workdir/primary/tls/consul-agent-ca.pem"
    cert_file              = "/workdir/primary/tls/primary-server-consul-0.pem"
    key_file               = "/workdir/primary/tls/primary-server-consul-0-key.pem"
    verify_incoming        = true
    verify_outgoing        = true
    verify_server_hostname = true
  }
}
