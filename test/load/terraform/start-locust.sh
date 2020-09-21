#!/bin/bash

# set up locust config
cat <<CONF > /home/ubuntu/locust.conf
locustfile = /scripts/puts_locustfile.py
worker = true
master-host = ${primary_ip}
host = http://${lb_endpoint}:8500
users = 100
spawn-rate = 10
run-time = 10m  
CONF

# run test
locust 
