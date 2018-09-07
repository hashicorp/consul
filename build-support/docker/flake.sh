#!/bin/bash
LOG_FILE="test.log"

cd $GOPATH/$APP

export PATH=$GOPATH/$APP/bin:$GOPATH/bin:$PATH

if ! [[ $(ls -l | grep 'GNUmakefile\|README.md\|LICENSE') ]] ; then
	echo "App source not present in cwd. Exiting..."
	exit 1
fi

mv $TEST_BINARY $TEST_PKG/$TEST_BINARY
cd $TEST_PKG

for ((i=0; i < $ITERATIONS; i++)) ; do
	echo "$(date +"%F %T") - ($((i+1))/$ITERATIONS)"
	
	./$TEST_BINARY -test.run "$TEST" -test.parallel 4 -test.timeout 8m -test.v &> $LOG_FILE
	echo $? > exit-code

	grep -A10 'panic: ' $LOG_FILE || true
	awk '/^[^[:space:]]/ {do_print=0} /--- SKIP/ {do_print=1} do_print==1 {print}' $LOG_FILE
	awk '/^[^[:space:]]/ {do_print=0} /--- FAIL/ {do_print=1} do_print==1 {print}' $LOG_FILE

	if [ $(cat exit-code) != "0" ] ; then
		exit 1;
	fi
done