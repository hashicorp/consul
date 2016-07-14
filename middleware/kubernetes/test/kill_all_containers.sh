#!/bin/bash

docker rm -f $(docker ps -a -q)
sleep 1
docker rm -f $(docker ps -a -q)
