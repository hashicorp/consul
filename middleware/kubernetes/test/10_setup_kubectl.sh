#!/bin/bash

PWD=`pwd`
BASEDIR=`realpath $(dirname ${0})`

cd ${BASEDIR}
if [ ! -e kubectl ]; then
	curl -O http://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl
	chmod u+x kubectl
fi

${BASEDIR}/kubectl config set-cluster test-doc --server=http://localhost:8080
${BASEDIR}/kubectl config set-context test-doc --cluster=test-doc
${BASEDIR}/kubectl config use-context test-doc

cd ${PWD}

alias kubctl="${BASEDIR}/kubectl"
