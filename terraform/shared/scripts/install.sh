#!/usr/bin/env bash
set -e

echo "Installing dependencies..."
if [ -x "$(command -v apt-get)" ]; then
  sudo su -s /bin/bash -c 'sleep 30 && apt-get update && apt-get -y install bsdtar' root
else
  sudo yum update -y
  sudo yum install -y bsdtar
fi

CONSUL_SERVER_COUNT=$1
CONSUL_VERSION=$2
CONSUL_BIND=$3
CONSUL_CLIENT_BIND=$4
CONSUL_TAG_JOIN=$5
CONSUL_TAG_VALUE=$6
CONSUL_DATACENTER=$7

echo "Fetching and installing Consul..."
cd /tmp
curl -s https://releases.hashicorp.com/consul/${CONSUL_VERSION}/consul_${CONSUL_VERSION}_linux_amd64.zip | bsdtar -xvf-

chmod +x consul
sudo mv consul /usr/local/bin/consul
sudo mkdir -p /opt/consul/data

# Read from the file we created
#SERVER_COUNT=$(cat /tmp/consul-server-count | tr -d '\n')
#CONSUL_JOIN=$(cat /tmp/consul-server-addr | tr -d '\n')

# Write the flags to a temporary file
cat >/tmp/consul_flags << EOF
CONSUL_UI_BETA=true
CONSUL_FLAGS="-server -ui -client ${CONSUL_CLIENT_BIND} -bind ${CONSUL_BIND} -bootstrap-expect=${CONSUL_SERVER_COUNT} -retry-join=\"provider=aws tag_key=${CONSUL_TAG_JOIN} tag_value=${CONSUL_TAG_VALUE}\" -data-dir=/opt/consul/data -datacenter ${CONSUL_DATACENTER}"
EOF

if [ -f /tmp/upstart.conf ];
then
  echo "Installing Upstart service..."
  sudo mkdir -p /etc/consul.d
  sudo mkdir -p /etc/service
  sudo chown root:root /tmp/upstart.conf
  sudo mv /tmp/upstart.conf /etc/init/consul.conf
  sudo chmod 0644 /etc/init/consul.conf
  sudo mv /tmp/consul_flags /etc/service/consul
  sudo chmod 0644 /etc/service/consul
else
  echo "Installing Systemd service..."
  sudo mkdir -p /etc/sysconfig
  sudo mkdir -p /etc/systemd/system/consul.d
  sudo chown root:root /tmp/consul.service
  sudo mv /tmp/consul.service /etc/systemd/system/consul.service
  sudo mv /tmp/consul*json /etc/systemd/system/consul.d/ || echo
  sudo chmod 0644 /etc/systemd/system/consul.service
  sudo mv /tmp/consul_flags /etc/sysconfig/consul
  sudo chown root:root /etc/sysconfig/consul
  sudo chmod 0644 /etc/sysconfig/consul
fi
