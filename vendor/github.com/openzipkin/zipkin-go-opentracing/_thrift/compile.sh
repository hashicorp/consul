#!/usr/bin/env sh

for f in *.thrift ; do
	thrift -r --gen go:thrift_import=github.com/apache/thrift/lib/go/thrift $f
done
