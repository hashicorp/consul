#!/usr/bin/env sh
CONSUL_CONFIG_DIR=${CONSUL_CONFIG_DIR:-/consul/config}

if [ -d "$CONSUL_CONFIG_DIR" ]; then
  if [ -n "$CONSUL_CA" ]; then
    echo "${CONSUL_CA}" > "$CONSUL_CONFIG_DIR/consul-agent-ca.pem"
  fi
  if [ -n "$CONSUL_CERT" ]; then
    echo "${CONSUL_CERT}" > "$CONSUL_CONFIG_DIR/consul-agent-0.pem"
  fi
  if [ -n "$CONSUL_KEY" ]; then
    echo "${CONSUL_KEY}" > "$CONSUL_CONFIG_DIR/consul-agent-0-key.pem"
  fi
  chown -R consul:consul "$CONSUL_CONFIG_DIR"
  chmod -R go-rwx "$CONSUL_CONFIG_DIR"
fi

docker-entrypoint.sh "$@"
