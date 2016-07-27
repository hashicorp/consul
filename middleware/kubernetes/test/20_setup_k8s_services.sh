#!/bin/bash

PWD=`pwd`
BASEDIR=`readlink -e $(dirname ${0})`

cd ${BASEDIR}

KUBECTL='./kubectl'

wait_until_k8s_ready() {
	# Wait until kubernetes is up and fully responsive
	while :
	do
   	    ${KUBECTL} get nodes 2>/dev/null | grep -q '127.0.0.1'
		if [ "${?}" = "0" ]; then
			break
		else
			echo "sleeping for 5 seconds (waiting for kubernetes to start)"
			sleep 5
		fi
	done
	echo "kubernetes nodes:"
	${KUBECTL} get nodes
}

create_namespaces() {
	for n in ${NAMESPACES};
	do
			echo "Creating namespace: ${n}"
			${KUBECTL} get namespaces --no-headers 2>/dev/null | grep -q ${n}
			if [ "${?}" != "0" ]; then
				${KUBECTL} create namespace ${n}
			fi
	done

	echo "kubernetes namespaces:"
	${KUBECTL} get namespaces
}

# run_and_expose_service <servicename> <namespace> <image> <port>
run_and_expose_service() {

    if [ "${#}" != "4" ]; then
        return -1
    fi

    service="${1}"
    namespace="${2}"
    image="${3}"
    port="${4}"

    echo "   starting service '${service}' in namespace '${namespace}"

    ${KUBECTL} get deployment --namespace=${namespace} --no-headers 2>/dev/null | grep -q ${service}
    if [ "${?}" != "0" ]; then
        ${KUBECTL} run ${service} --namespace=${namespace} --image=${image}
    else
        echo "warn: service '${service}' already running in namespace '${namespace}'"
    fi

    ${KUBECTL} get service --namespace=${namespace} --no-headers 2>/dev/null | grep -q ${service}
    if [ "${?}" != "0" ]; then
        ${KUBECTL} expose deployment ${service} --namespace=${namespace} --port=${port}
    else
        echo "warn: service '${service}' already exposed in namespace '${namespace}'"
    fi
}


wait_until_k8s_ready

NAMESPACES="demo test"
create_namespaces

echo ""
echo "Starting services:"

run_and_expose_service mynginx demo nginx 80
run_and_expose_service webserver demo nginx 80
run_and_expose_service mynginx test nginx 80
run_and_expose_service webserver test nginx 80

echo ""
echo "Services exposed:"
${KUBECTL} get services --all-namespaces

cd ${PWD}
