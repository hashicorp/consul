#!/bin/sh

set -e

if [ ! -d specifications ]; then
	git clone -b bson git@github.com:jyemin/specifications
fi

TESTFILE="../specdata_test.go"

cat <<END > $TESTFILE
package bson_test

var specTests = []string{
END

for file in specifications/source/bson/tests/*.yml; do
	(
		echo '`'
		cat $file
		echo -n '`,'
	) >> $TESTFILE
done

echo '}' >> $TESTFILE

gofmt -w $TESTFILE
