#!/bin/bash

PWD=`pwd`
BASEDIR=`readlink -e $(dirname ${0})`

cd ${BASEDIR}

./00_run_k8s.sh && \
./10_setup_kubectl.sh && \
./20_setup_k8s_services.sh

cd ${PWD}
