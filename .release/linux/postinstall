#!/bin/bash

mkdir -p /opt/consul
chown -R consul:consul /opt/consul
chown -R consul:consul /etc/consul.d

if [ -d /run/systemd/system ]; then
    systemctl --system daemon-reload >/dev/null || true
fi

