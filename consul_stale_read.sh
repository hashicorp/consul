#!/bin/bash

consul_dir="consul.foo"
consul_cmd="consul agent -data-dir consul.foo -client 127.0.0.1 -bind 127.0.0.1 -server -bootstrap-expect 1"
consul_log="consul.out"

# ensure consul isn't running
pkill consul

echo "About to remove consul dir"
# clean persistence and start consul
rm -rf "$consul_dir"
$consul_cmd  > "$consul_log"  2>&1 &
pid=$!
sleep 2

# Run test:
# Write a value to a test key, then read it out and check if it matches. Restart
# consul in between the write and read.
i="0"
prev_raft_info=""
while true; do
    # Write the value of 'i' to the test key
    ret="$(curl -s -X PUT -d "$i" http://127.0.0.1:8500/v1/kv/test)"
    if [ "$ret" != "true" ]; then
        echo "Failed to write key, try increasing the sleep time after the initial consul start ($ret)"
        exit 1
    fi

    # Restart consul
    kill -9 $pid
    wait
    $consul_cmd  > "$consul_log.$i"  2>&1 &
    pid=$!

    # Read the value of 'i' from the test key, wait for consul to return a valid response
    val=""
    json=""
    raft_info=""
    while [ "$json" == "No cluster leader" ] || [ "$json" == "" ]; do
        #get raft information
        json="$(curl -X GET 'http://127.0.0.1:8500/v1/kv/test?consistent')"
        if [ "$json" == "No cluster leader" ] || [ "$json" == "" ] 
        then 
             continue
        fi
        raft_info=`consul info`
        val=`echo $json | jq ".[0].Value" | sed 's/"//g' | base64 -d`
        mod_index=`echo $json | jq ".[0].ModifyIndex"`
        echo "read value $val at modify index $val in iteration $i"
        #echo "RAFT info - commit index is $commit_index, applied is $applied_index, last log index $last_log_index, last snapshot index $last_snapshot_index"
    done

    # ensure read value matches what we wrote
    if (( val != i )); then
        echo "Incorrect value read, expected $i found $val"
        echo "RAFT INFO\\n$raft_info"
        echo "PREV RAFT INFO\\n$prev_raft_info"
        exit -1
    fi

    # increment 'i'
    i=$[$i+1]
    if (( i % 10 == 0 )); then
        echo "Iteration $i"
    fi
    prev_raft_info=$raft_info
done
