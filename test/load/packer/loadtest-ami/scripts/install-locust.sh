#!/bin/bash -e
apt update && apt install -y python3 python3-pip
pip3 install --upgrade pip
pip3 install locust


# set new limit
echo "fs.file-max = 2097152" >> /etc/sysctl.conf
ulimit -Sn 100000
sysctl -p