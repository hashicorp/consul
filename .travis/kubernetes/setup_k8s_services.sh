#!/bin/bash

set -x

KUBECTL='docker exec hyperkube /hyperkube kubectl'

PWD=`pwd`
cd `readlink -e $(dirname ${0})`

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

	echo "   starting service '${service}' in namespace '${namespace}'"

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

#run_and_expose_rc nginx-controller nginx-rc.yml poddemo 80
run_and_expose_rc() {
	if [ "${#}" != "4" ]; then
		return -1
	fi

	rc_name="${1}"
	rc_file="${2}"
	namespace="${3}"
	port="${4}"

	echo "   starting replication controller '${rc_name}' from '${rc_file}' in namespace '${namespace}'"

	${KUBECTL} get rc --namespace=${namespace} --no-headers 2>/dev/null | grep -q ${rc_name}
	if [ "${?}" != "0" ]; then
		${KUBECTL} expose -f ${rc_file} --namespace=${namespace} --port=${port}
	else
		echo "warn: rc '${rc_name}' already running in namespace '${namespace}'"
	fi
}

echo "Starting sample kubernetes services..."

NAMESPACES="demo poddemo test"
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

echo ""
echo "Starting replicationcontrollers:"

run_and_expose_rc nginx-controller nginx-rc.yml poddemo 80

echo ""
echo "ReplicationControllers exposed:"
${KUBECTL} get rc --all-namespaces

cd ${PWD}
