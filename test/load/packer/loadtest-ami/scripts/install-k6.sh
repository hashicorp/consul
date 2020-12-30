#!/bin/bash -e

# set new limit
echo "fs.file-max = 2097152" >> /etc/sysctl.conf
ulimit -Sn 100000
sysctl -p

# download k6
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 379CE192D401AB61
echo "deb https://dl.bintray.com/loadimpact/deb stable main" | sudo tee -a /etc/apt/sources.list
sudo apt-get update
sudo apt-get install k6

# move service file
chmod 755 /home/ubuntu/scripts/loadtest.js