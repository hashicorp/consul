#!/usr/bin/env bash

FILENAME=$3
echo $PWD
if [[ "$FILENAME" =~ .*pbcommon/.* ]]; then
    echo "$FILENAME no gogo"
    ./build-support/scripts/proto-gen-no-gogo.sh $1 $2 $3
elif [[ "$FILENAME" =~ .*pbconnect/.* ]]; then
    echo "$FILENAME no gogo"
        ./build-support/scripts/proto-gen-no-gogo.sh $1 $2 $3
elif [[ "$FILENAME" =~ .*pbconfig/.* ]]; then
    echo "$FILENAME no gogo"
        ./build-support/scripts/proto-gen-no-gogo.sh $1 $2 $3
elif [[ "$FILENAME" =~ .*pbautoconf/.* ]]; then
    echo "$FILENAME no gogo"
        ./build-support/scripts/proto-gen-no-gogo.sh $1 $2 $3
else
    echo "$FILENAME gogo"
    ./build-support/scripts/proto-gen.sh $1 $2 $3
fi