#!/bin/bash

echo "LB_ENDPOINT=${lb_endpoint}" >> /etc/environment

systemctl start loadtest
