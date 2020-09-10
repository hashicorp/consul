#!/bin/bash -e
## Datadog doesn't exist yet
mkdir -p /etc/datadog-agent/conf.d/consul.d/


##Move datadog files
mv /home/ubuntu/scripts/conf.yaml /etc/datadog-agent/conf.d/consul.d/
mv /home/ubuntu/scripts/datadog.yaml /etc/datadog-agent/

##Move Consul Config that hooks up to datadog
mv /home/ubuntu/scripts/telemetry.json /opt/consul/config/
chown consul:consul /opt/consul/config/telemetry.json
