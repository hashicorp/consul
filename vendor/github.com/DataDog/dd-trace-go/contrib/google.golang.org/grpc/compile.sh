#!/bin/bash

# compiles test fixtures
set -e
protoc -I . fixtures.proto --go_out=plugins=grpc:.

# FIXME[matt] hacks to move the fixtures into the testing package
# and make it pass our lint rules. This is cheesy but very simple.
mv fixtures.pb.go fixtures_test.go
sed -i 's/_Fixture_Ping_Handler/fixturePingHandler/' fixtures_test.go
sed -i 's/_Fixture_serviceDesc/fixtureServiceDesc/' fixtures_test.go
