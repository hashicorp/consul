#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

#
# Script for bringing up an N node consul cluster
# on the local machine on different ports.
#
# The first node is listening on the default ports
# so that the command line tool works.
#
# Examples:
#
# 3-node cluster:
#
#  $ consul-cluster.bash
#  $ consul-cluster.bash 3
#
# 5-node cluster with specific consul version:
#
#  $ consul-cluster.bash 5 ~/consul-0.7.5/consul

config() {
	local port=${1:-0}
	local name="consul${port}"
	local nodeid=$(printf "00000000-0000-0000-0000-%012d" $port)
	local path="$DIR/${name}"

	cat << EOF > "${path}/a.json"
{
	"server"           : true,
	"node_id"          : "${nodeid}",
	"node_name"        : "${name}",
	"data_dir"         : "${name}/data",
	"pid_file"         : "${name}/pid",
	"bind_addr"        : "127.0.0.1",
	"retry_join"       : ["127.0.0.1:8301","127.0.0.1:8304","127.0.0.1:8307"],
	"bootstrap_expect" : ${N},
	"ports" : {
		"http"     : $((8500 + $port)),
		"dns"      : $((8600 + $port)),
		"server"   : $((8300 + 3*$port)),
		"serf_lan" : $((8301 + 3*$port)),
		"serf_wan" : $((8302 + 3*$port)),
		"rpc"      : $((8400 + $port))
	}
}
EOF
}

trap cleanup EXIT TERM KILL

jobs=
cleanup() {
	[ "$jobs" == "" ] || kill $jobs
	[ "$CLEANDIR" == "y" -a "$DIR" != "" ] && rm -rf "$DIR"
}

run() {
	local port=$1
	local name=consul${port}
	local path="$DIR/${name}"

	rm -rf "${path}"
	mkdir -p "${path}"
	config $port
	( $CONSUL agent -config-dir "${path}" 2>&1 | tee "${path}/log" ; echo "Exit code: $?" >> "${path}/log" ) &
	jobs="$jobs $!"
}

N=3
CONSUL=$(which consul)
CLEANDIR=y
SLEEP=y

while test $# -gt 0 ; do
	case "$1" in
		-d|--dir)
			shift
			DIR=$1
			CLEANDIR=n
			shift
			;;
		-n|--nodes)
			shift
			N=$1
			shift
			;;
		-q|--quick)
			shift
			SLEEP=n
			;;
		-x|--consul)
			shift
			CONSUL=$1
			shift
			;;
		*)
			echo "Usage: $(basename $0) [-n nodes] [-x consul] [-d dir]"
			echo ""
			echo " -h, --help            brief help"
			echo " -d, --dir temp dir    path to the temp directory, default is $DIR"
			echo " -n, --nodes nodes     number of nodes to start, default is $N"
			echo " -q, --quick           do not wait during startup"
			echo " -x, --consul consul   consul binary, default is $CONSUL"
			exit 0
			;;
	esac
done

[ "$DIR" == "" ] && DIR=$(mktemp -d /tmp/consul-cluster-XXXXXXX)

echo "Starting $N node cluster. exe=$CONSUL data=$DIR"
[ "$CLEANDIR" == "y" ] && echo "Data files will be removed"
echo "Stop with CTRL-C"
echo ""
[ "$SLEEP" == "y" ] && sleep 3

for ((i=0 ; i < N ; i++)) ; do run $i ; done

wait
