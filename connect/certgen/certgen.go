// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// certgen: a tool for generating test certificates on disk for use as
// test-fixtures and for end-to-end testing and local development.
//
// Example usage:
//
//	$ go run connect/certgen/certgen.go -out-dir /tmp/connect-certs
//
// You can verify a given leaf with a given root using:
//
//	$ openssl verify -verbose -CAfile ca1-ca.cert.pem ca1-svc-db.cert.pem
//
// Note that to verify via the cross-signed intermediate, openssl requires it to
// be bundled with the _root_ CA bundle and will ignore the cert if it's passed
// with the subject. You can do that with:
//
//	$ openssl verify -verbose -CAfile \
//	   <(cat ca1-ca.cert.pem ca2-xc-by-ca1.cert.pem) \
//	   ca2-svc-db.cert.pem
//	ca2-svc-db.cert.pem: OK
//
// Note that the same leaf and root without the intermediate should fail:
//
//	$ openssl verify -verbose -CAfile ca1-ca.cert.pem ca2-svc-db.cert.pem
//	ca2-svc-db.cert.pem: CN = db
//	error 20 at 0 depth lookup:unable to get local issuer certificate
//
// NOTE: THIS IS A QUIRK OF OPENSSL; in Connect we distribute the roots alone
// and stable intermediates like the XC cert to the _leaf_.
package main // import "github.com/hashicorp/consul/connect/certgen"

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

func main() {
	var numCAs = 2
	var services = []string{"web", "db", "cache"}
	var outDir string
	var keyType string = "ec"
	var keyBits int = 256

	flag.StringVar(&outDir, "out-dir", "",
		"REQUIRED: the dir to write certificates to")
	flag.StringVar(&keyType, "key-type", "ec",
		"Type of private key to create (ec, rsa)")
	flag.IntVar(&keyBits, "key-bits", 256,
		"Size of private key to create, in bits")
	flag.Parse()

	if outDir == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create CA certs
	var prevCA *structs.CARoot
	for i := 1; i <= numCAs; i++ {
		ca := connect.TestCAWithKeyType(&testing.RuntimeT{}, prevCA, keyType, keyBits)
		prefix := fmt.Sprintf("%s/ca%d-ca", outDir, i)
		writeFile(prefix+".cert.pem", ca.RootCert)
		writeFile(prefix+".key.pem", ca.SigningKey)
		if prevCA != nil {
			fname := fmt.Sprintf("%s/ca%d-xc-by-ca%d.cert.pem", outDir, i, i-1)
			writeFile(fname, ca.SigningCert)
		}
		prevCA = ca

		// Create service certs for each CA
		for _, svc := range services {
			certPEM, keyPEM := connect.TestLeaf(&testing.RuntimeT{}, svc, ca)
			prefix := fmt.Sprintf("%s/ca%d-svc-%s", outDir, i, svc)
			writeFile(prefix+".cert.pem", certPEM)
			writeFile(prefix+".key.pem", keyPEM)
		}
	}
}

func writeFile(name, content string) {
	fmt.Println("Writing ", name)
	err := os.WriteFile(name, []byte(content), 0600)
	if err != nil {
		log.Fatalf("failed writing file: %s", err)
	}
}
