#!/bin/bash

# set up locust config
cat <<CONF > /home/ubuntu/locust.conf
locustfile = /home/ubuntu/scripts/puts_locustfile.py
worker = true
master-host = ${primary_ip}
host = http://${lb_endpoint}:8500
CONF

sleep 60
systemctl start loadtest