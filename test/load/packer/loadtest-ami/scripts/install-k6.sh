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
mv /home/ubuntu/scripts/loadtest.service /etc/systemd/system/loadtest.service
chmod 755 /home/ubuntu/scripts/puts_script.js
chmod 755 /home/ubuntu/scripts/run-k6.sh
