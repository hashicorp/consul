package consul

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

func TestConfig_AppendCA_None(t *testing.T) {
	conf := &Config{}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(pool.Subjects()) != 0 {
		t.Fatalf("bad: %v", pool.Subjects())
	}
}

func TestConfig_CACertificate_Valid(t *testing.T) {
	conf := &Config{
		CAFile: "../test/ca/root.cer",
	}
	pool := x509.NewCertPool()
	err := conf.AppendCA(pool)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(pool.Subjects()) == 0 {
		t.Fatalf("expected cert")
	}
}

func TestConfig_KeyPair_None(t *testing.T) {
	conf := &Config{}
	cert, err := conf.KeyPair()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cert != nil {
		t.Fatalf("bad: %v", cert)
	}
}

func TestConfig_KeyPair_Valid(t *testing.T) {
	conf := &Config{
		CertFile: "../test/key/ourdomain.cer",
		KeyFile:  "../test/key/ourdomain.key",
	}
	cert, err := conf.KeyPair()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cert == nil {
		t.Fatalf("expected cert")
	}
}

func TestConfig_OutgoingTLS_MissingCA(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
	}
	tls, err := conf.OutgoingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
	if tls != nil {
		t.Fatalf("bad: %v", tls)
	}
}

func TestConfig_OutgoingTLS_OnlyCA(t *testing.T) {
	conf := &Config{
		CAFile: "../test/ca/root.cer",
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls != nil {
		t.Fatalf("expected no config")
	}
}

func TestConfig_OutgoingTLS_VerifyOutgoing(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls == nil {
		t.Fatalf("expected config")
	}
	if len(tls.RootCAs.Subjects()) != 1 {
		t.Fatalf("expect root cert")
	}
	if tls.ServerName != "" {
		t.Fatalf("expect no server name verification")
	}
	if !tls.InsecureSkipVerify {
		t.Fatalf("should skip built-in verification")
	}
}

func TestConfig_OutgoingTLS_ServerName(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		ServerName:     "consul.example.com",
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls == nil {
		t.Fatalf("expected config")
	}
	if len(tls.RootCAs.Subjects()) != 1 {
		t.Fatalf("expect root cert")
	}
	if tls.ServerName != "consul.example.com" {
		t.Fatalf("expect server name")
	}
	if tls.InsecureSkipVerify {
		t.Fatalf("should not skip built-in verification")
	}
}

func TestConfig_OutgoingTLS_WithKeyPair(t *testing.T) {
	conf := &Config{
		VerifyOutgoing: true,
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	tls, err := conf.OutgoingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tls == nil {
		t.Fatalf("expected config")
	}
	if len(tls.RootCAs.Subjects()) != 1 {
		t.Fatalf("expect root cert")
	}
	if !tls.InsecureSkipVerify {
		t.Fatalf("should skip verification")
	}
	if len(tls.Certificates) != 1 {
		t.Fatalf("expected client cert")
	}
}

func TestConfig_IncomingTLS(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CAFile:         "../test/ca/root.cer",
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	tlsC, err := conf.IncomingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tlsC == nil {
		t.Fatalf("expected config")
	}
	if len(tlsC.ClientCAs.Subjects()) != 1 {
		t.Fatalf("expect client cert")
	}
	if tlsC.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("should not skip verification")
	}
	if len(tlsC.Certificates) != 1 {
		t.Fatalf("expected client cert")
	}
}

func TestConfig_IncomingTLS_MissingCA(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CertFile:       "../test/key/ourdomain.cer",
		KeyFile:        "../test/key/ourdomain.key",
	}
	_, err := conf.IncomingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestConfig_IncomingTLS_MissingKey(t *testing.T) {
	conf := &Config{
		VerifyIncoming: true,
		CAFile:         "../test/ca/root.cer",
	}
	_, err := conf.IncomingTLSConfig()
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestConfig_IncomingTLS_NoVerify(t *testing.T) {
	conf := &Config{}
	tlsC, err := conf.IncomingTLSConfig()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tlsC == nil {
		t.Fatalf("expected config")
	}
	if len(tlsC.ClientCAs.Subjects()) != 0 {
		t.Fatalf("do not expect client cert")
	}
	if tlsC.ClientAuth != tls.NoClientCert {
		t.Fatalf("should skip verification")
	}
	if len(tlsC.Certificates) != 0 {
		t.Fatalf("unexpected client cert")
	}
}
