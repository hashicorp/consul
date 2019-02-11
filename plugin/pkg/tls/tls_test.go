package tls

import (
	"path/filepath"
	"testing"

	"github.com/coredns/coredns/plugin/test"
)

func getPEMFiles(t *testing.T) (rmFunc func(), cert, key, ca string) {
	tempDir, rmFunc, err := test.WritePEMFiles("")
	if err != nil {
		t.Fatalf("Could not write PEM files: %s", err)
	}

	cert = filepath.Join(tempDir, "cert.pem")
	key = filepath.Join(tempDir, "key.pem")
	ca = filepath.Join(tempDir, "ca.pem")

	return
}

func TestNewTLSConfig(t *testing.T) {
	rmFunc, cert, key, ca := getPEMFiles(t)
	defer rmFunc()

	_, err := NewTLSConfig(cert, key, ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
}

func TestNewTLSClientConfig(t *testing.T) {
	rmFunc, _, _, ca := getPEMFiles(t)
	defer rmFunc()

	_, err := NewTLSClientConfig(ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
}

func TestNewTLSConfigFromArgs(t *testing.T) {
	rmFunc, cert, key, ca := getPEMFiles(t)
	defer rmFunc()

	_, err := NewTLSConfigFromArgs()
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}

	c, err := NewTLSConfigFromArgs(ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs == nil {
		t.Error("RootCAs should not be nil when one arg passed")
	}

	c, err = NewTLSConfigFromArgs(cert, key)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs != nil {
		t.Error("RootCAs should be nil when two args passed")
	}
	if len(c.Certificates) != 1 {
		t.Error("Certificates should have a single entry when two args passed")
	}
	args := []string{cert, key, ca}
	c, err = NewTLSConfigFromArgs(args...)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}
	if c.RootCAs == nil {
		t.Error("RootCAs should not be nil when three args passed")
	}
	if len(c.Certificates) != 1 {
		t.Error("Certificates should have a single entry when three args passed")
	}
}

func TestNewHTTPSTransport(t *testing.T) {
	rmFunc, _, _, ca := getPEMFiles(t)
	defer rmFunc()

	cc, err := NewTLSClientConfig(ca)
	if err != nil {
		t.Errorf("Failed to create TLSConfig: %s", err)
	}

	tr := NewHTTPSTransport(cc)
	if tr == nil {
		t.Errorf("Failed to create https transport with cc")
	}

	tr = NewHTTPSTransport(nil)
	if tr == nil {
		t.Errorf("Failed to create https transport without cc")
	}
}
