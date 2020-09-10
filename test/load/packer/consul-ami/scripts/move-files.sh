#!/bin/bash -e

##Move datadog files, set USER:GROUP
mv /home/ubuntu/scripts/conf.yaml /etc/datadog-agent/conf.d/consul.d/
chown dd-agent:dd-agent /etc/datadog-agent/conf.d/consul.d/conf.yaml
mv /home/ubuntu/scripts/datadog.yaml /etc/datadog-agent/
chown dd-agent:dd-agent /etc/datadog-agent/datadog.yaml

##Move Consul Config that hooks up to datadog
mv /home/ubuntu/scripts/telemetry.json /opt/consul/config/
chown consul:consul /opt/consul/config/telemetry.json