#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# SOURCE: GRUNTWORKS
# This script is meant to be run in the User Data of each EC2 Instance while it's booting. The script uses the
# run-consul script to configure and start Consul in server mode. Note that this script assumes it's running in an AMI
# built from the Packer template in examples/consul-ami/consul.json.

set -e

# Send the log output from this script to user-data.log, syslog, and the console
# From: https://alestic.com/2010/12/ec2-user-data-output/
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1

# Install Consul
if [[ -n "${consul_download_url}" ]]; then
    /home/ubuntu/scripts/install-consul --download-url "${consul_download_url}"
else
    /home/ubuntu/scripts/install-consul --version "${consul_version}"
fi

# Update User:Group on this file really quick
chown consul:consul /opt/consul/config/telemetry.json

# These variables are passed in via Terraform template interplation
/opt/consul/bin/run-consul --server --cluster-tag-key "${cluster_tag_key}" --cluster-tag-value "${cluster_tag_value}"
