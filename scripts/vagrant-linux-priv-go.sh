#!/usr/bin/env bash

function install_go() {
	local go_version=1.9.1
	local download=

	download="https://storage.googleapis.com/golang/go${go_version}.linux-amd64.tar.gz"

	if [ -d /usr/local/go ] ; then
		return
	fi

	wget -q -O /tmp/go.tar.gz ${download}

	tar -C /tmp -xf /tmp/go.tar.gz
	sudo mv /tmp/go /usr/local
	sudo chown -R root:root /usr/local/go
}

install_go

# Ensure that the GOPATH tree is owned by vagrant:vagrant
mkdir -p /opt/gopath
chown -R vagrant:vagrant /opt/gopath

# Ensure Go is on PATH
if [ ! -e /usr/bin/go ] ; then
	ln -s /usr/local/go/bin/go /usr/bin/go
fi
if [ ! -e /usr/bin/gofmt ] ; then
	ln -s /usr/local/go/bin/gofmt /usr/bin/gofmt
fi


# Ensure new sessions know about GOPATH
if [ ! -f /etc/profile.d/gopath.sh ] ; then
	cat <<EOT > /etc/profile.d/gopath.sh
export GOPATH="/opt/gopath"
export PATH="/opt/gopath/bin:\$PATH"
EOT
	chmod 755 /etc/profile.d/gopath.sh
fi
