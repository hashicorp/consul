#!/bin/bash
set -e

# Read the address to join from the file we provisioned
JOIN_ADDRS=$(cat /tmp/consul-server-addr | tr -d '\n')

echo "Installing dependencies..."
yum update -y
yum install -y unzip wget

echo "Fetching Consul..."
cd /tmp
wget https://dl.bintray.com/mitchellh/consul/0.5.2_linux_amd64.zip -O consul.zip

echo "Installing Consul..."
unzip consul.zip >/dev/null
chmod +x consul
mv consul /usr/local/bin/consul
mkdir -p /etc/consul.d
mkdir -p /mnt/consul
mkdir -p /etc/service

#Enable consul port in iptables
echo "Allow port 8301 in iptables"
iptables -I INPUT -s 0/0 -p tcp --dport 8301 -j ACCEPT

# Setup the join address
cat >/tmp/consul-join << EOF
export CONSUL_JOIN="${JOIN_ADDRS}"
EOF
mv /tmp/consul-join /etc/service/consul-join
chmod 0644 /etc/service/consul-join

echo "Installing Upstart service..."
mv /tmp/upstart.conf /etc/init/consul.conf
mv /tmp/upstart-join.conf /etc/init/consul-join.conf
