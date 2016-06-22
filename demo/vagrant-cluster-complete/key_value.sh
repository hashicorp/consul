#!/bin/bash
#
# Consul Key-Value demonstration
#
# This script duplicates the actions given at
# https://www.consul.io/intro/getting-started/kv.html.  Ssh into the
# virtual box (either one) and run the following:
#
#   ./vagrant/key_value.sh
#
# To delete all keys, add "delete" as an input arg:
#
#   ./vagrant/key_value.sh delete


if [ "$1" == "delete" ]; then
    curl --silent -X DELETE http://localhost:8500/v1/kv/?recurse > /dev/null
    echo "All keys deleted."
    exit 0
fi


set -v

# First verify that there are no existing keys in the k/v store:
curl -v http://localhost:8500/v1/kv/?recurse

# PUT some keys:
curl -X PUT -d 'test' http://localhost:8500/v1/kv/web/key1

curl -X PUT -d 'test' http://localhost:8500/v1/kv/web/key2?flags=42

curl -X PUT -d 'test' http://localhost:8500/v1/kv/web/sub/key3


# Now there are keys returned:
curl http://localhost:8500/v1/kv/?recurse | python -mjson.tool

# Fetch single key:
curl http://localhost:8500/v1/kv/web/key1 | python -mjson.tool

# Delete key (here, deleting recursively):
curl -X DELETE http://localhost:8500/v1/kv/web/sub?recurse

curl http://localhost:8500/v1/kv/?recurse | python -mjson.tool

# Update a key.
set +v
# Ref http://www.cambus.net/parsing-json-from-command-line-using-python/
lastindex=`curl --silent http://localhost:8500/v1/kv/web/key1 | python -c 'import sys, json; print json.load(sys.stdin)[0]["ModifyIndex"]'`
set -v
curl -X PUT -d 'new_value' http://localhost:8500/v1/kv/web/key1?cas=${lastindex}

curl -X PUT -d 'rejected_update' http://localhost:8500/v1/kv/web/key1?cas=${lastindex}

set +v
value=`curl --silent http://localhost:8500/v1/kv/web/key1 | python -c 'import sys, json; print json.load(sys.stdin)[0]["Value"]' | base64 --decode`

echo "New key value: ${value}"
set -v

