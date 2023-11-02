// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

// same certificate that appears in our connect tests
var validCA = `
-----BEGIN CERTIFICATE-----
MIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg
Q0EgNzAeFw0xODA1MjExNjMzMjhaFw0yODA1MTgxNjMzMjhaMBYxFDASBgNVBAMT
C0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAER0qlxjnRcMEr
iSGlH7G7dYU7lzBEmLUSMZkyBbClmyV8+e8WANemjn+PLnCr40If9cmpr7RnC9Qk
GTaLnLiF16OCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/
MGgGA1UdDgRhBF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1OTpjMjpmYTo0ZTo3
NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToyNDpiMDowNDpiMzpl
ODo5Nzo1Yjo3ZTBqBgNVHSMEYzBhgF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1
OTpjMjpmYTo0ZTo3NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToy
NDpiMDowNDpiMzplODo5Nzo1Yjo3ZTA/BgNVHREEODA2hjRzcGlmZmU6Ly8xMjRk
ZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIuY29uc3VsMD0GA1UdHgEB
/wQzMDGgLzAtgisxMjRkZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIu
Y29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIQDzkkI7R+0U12a+zq2EQhP/n2mHmta+
fs2hBxWIELGwTAIgLdO7RRw+z9nnxCIA6kNl//mIQb+PGItespiHZKAz74Q=
-----END CERTIFICATE-----
`
var invalidCA = `
-----BEGIN CERTIFICATE-----
not valid
-----END CERTIFICATE-----
`

var validAddress = "1.2.3.4:80"
var validHostnameAddress = "foo.bar.baz:80"

var validServerName = "server.consul"

var validPeerID = "peer1"

// TODO(peering): the test methods below are exposed to prevent duplication,
// these should be removed at same time tests in peering_test get refactored.
// XXX: we can't put the existing tests in service_test.go into the peering
// package because it causes an import cycle by importing the top-level consul
// package (which correctly imports the agent/rpc/peering package)

// TestPeering is a test utility for generating a pbpeering.Peering with valid
// data along with the peerName, state and index.
func TestPeering(peerName string, state pbpeering.PeeringState, meta map[string]string) *pbpeering.Peering {
	return &pbpeering.Peering{
		Name:                peerName,
		PeerCAPems:          []string{validCA},
		PeerServerAddresses: []string{validAddress},
		PeerServerName:      validServerName,
		State:               state,
		PeerID:              validPeerID,
		Meta:                meta,
		Partition:           acl.DefaultPartitionName,
	}
}

// TestPeeringToken is a test utility for generating a valid peering token
// with the given peerID for use in test cases
func TestPeeringToken(peerID string) structs.PeeringToken {
	return structs.PeeringToken{
		CA:              []string{validCA},
		ServerAddresses: []string{validAddress},
		ServerName:      validServerName,
		PeerID:          peerID,
	}
}
