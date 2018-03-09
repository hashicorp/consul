#! /bin/bash

N=0
MAX_RETRY=10

docker ps --no-trunc

until [ $N -ge $MAX_RETRY ]
do
    TIMES=$[$N+1]
    echo "Contacting ElasticSearch... $TIMES/$MAX_RETRY"
    curl -XGET 'localhost:59200' && break
    N=$[$N+1]
    sleep 15
done
