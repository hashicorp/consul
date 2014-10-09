#!/bin/bash

grep generateUUID consul/state_store.go
RESULT=$?
if [ $RESULT -eq 0 ]; then
    exit 1
fi

grep generateUUID consul/fsm.go
RESULT=$?
if [ $RESULT -eq 0 ]; then
    exit 1
fi
