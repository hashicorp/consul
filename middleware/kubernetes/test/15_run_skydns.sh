#!/bin/bash

# Running skydns based on instructions at: https://testdatamanagement.wordpress.com/2015/09/01/running-kubernetes-in-docker-with-dns-on-a-single-node/

KUBECTL='./kubectl'

#RUN_SKYDNS="yes"
RUN_SKYDNS="no"

wait_until_k8s_ready() {
	# Wait until kubernetes is up and fully responsive
	while :
	do
   	 ${KUBECTL} get nodes 2>/dev/null | grep -q '127.0.0.1'
		if [ "${?}" = "0" ]; then
			break
		else
			echo "sleeping for 5 seconds"
			sleep 5
		fi
	done
	echo "kubernetes nodes:"
	${KUBECTL} get nodes
}


if [ "${RUN_SKYDNS}" = "yes" ]; then
	wait_until_k8s_ready

	echo "Launch kube2sky..."
	docker run -d --net=host gcr.io/google_containers/kube2sky:1.11 --kube_master_url=http://127.0.0.1:8080 --domain=cluster.local

	echo ""

	echo "Launch SkyDNS..."
	docker run -d --net=host gcr.io/google_containers/skydns:2015-03-11-001 --machines=http://localhost:4001 --addr=0.0.0.0:53 --domain=cluster.local
else
	true
fi
