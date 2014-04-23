package agent

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestConfigEncryptBytes(t *testing.T) {
	// Test with some input
	src := []byte("abc")
	c := &Config{
		EncryptKey: base64.StdEncoding.EncodeToString(src),
	}

	result, err := c.EncryptBytes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(src, result) {
		t.Fatalf("bad: %#v", result)
	}

	// Test with no input
	c = &Config{}
	result, err = c.EncryptBytes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) > 0 {
		t.Fatalf("bad: %#v", result)
	}
}

func TestDecodeConfig(t *testing.T) {
	// Basics
	input := `{"data_dir": "/tmp/", "log_level": "debug"}`
	config, err := DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.DataDir != "/tmp/" {
		t.Fatalf("bad: %#v", config)
	}

	if config.LogLevel != "debug" {
		t.Fatalf("bad: %#v", config)
	}

	// Without a protocol
	input = `{"node_name": "foo", "datacenter": "dc2"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "foo" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Datacenter != "dc2" {
		t.Fatalf("bad: %#v", config)
	}

	if config.SkipLeaveOnInt != DefaultConfig().SkipLeaveOnInt {
		t.Fatalf("bad: %#v", config)
	}

	if config.LeaveOnTerm != DefaultConfig().LeaveOnTerm {
		t.Fatalf("bad: %#v", config)
	}

	// Server bootstrap
	input = `{"server": true, "bootstrap": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !config.Server {
		t.Fatalf("bad: %#v", config)
	}

	if !config.Bootstrap {
		t.Fatalf("bad: %#v", config)
	}

	// DNS setup
	input = `{"ports": {"dns": 8500}, "recursor": "8.8.8.8", "domain": "foobar"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.Ports.DNS != 8500 {
		t.Fatalf("bad: %#v", config)
	}

	if config.DNSRecursor != "8.8.8.8" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Domain != "foobar" {
		t.Fatalf("bad: %#v", config)
	}

	// RPC configs
	input = `{"ports": {"http": 1234, "rpc": 8100}, "client_addr": "0.0.0.0"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.ClientAddr != "0.0.0.0" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.HTTP != 1234 {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.RPC != 8100 {
		t.Fatalf("bad: %#v", config)
	}

	// Serf configs
	input = `{"ports": {"serf_lan": 1000, "serf_wan": 2000}}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.Ports.SerfLan != 1000 {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.SerfWan != 2000 {
		t.Fatalf("bad: %#v", config)
	}

	// Server addrs
	input = `{"ports": {"server": 8000}, "bind_addr": "127.0.0.2", "advertise_addr": "127.0.0.3"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.BindAddr != "127.0.0.2" {
		t.Fatalf("bad: %#v", config)
	}

	if config.AdvertiseAddr != "127.0.0.3" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Ports.Server != 8000 {
		t.Fatalf("bad: %#v", config)
	}

	// leave_on_terminate
	input = `{"leave_on_terminate": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.LeaveOnTerm != true {
		t.Fatalf("bad: %#v", config)
	}

	// skip_leave_on_interrupt
	input = `{"skip_leave_on_interrupt": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.SkipLeaveOnInt != true {
		t.Fatalf("bad: %#v", config)
	}

	// enable_debug
	input = `{"enable_debug": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.EnableDebug != true {
		t.Fatalf("bad: %#v", config)
	}

	// TLS
	input = `{"verify_incoming": true, "verify_outgoing": true}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.VerifyIncoming != true {
		t.Fatalf("bad: %#v", config)
	}

	if config.VerifyOutgoing != true {
		t.Fatalf("bad: %#v", config)
	}

	// TLS keys
	input = `{"ca_file": "my/ca/file", "cert_file": "my.cert", "key_file": "key.pem"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.CAFile != "my/ca/file" {
		t.Fatalf("bad: %#v", config)
	}
	if config.CertFile != "my.cert" {
		t.Fatalf("bad: %#v", config)
	}
	if config.KeyFile != "key.pem" {
		t.Fatalf("bad: %#v", config)
	}

	// Start join
	input = `{"start_join": ["1.1.1.1", "2.2.2.2"]}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.StartJoin) != 2 {
		t.Fatalf("bad: %#v", config)
	}
	if config.StartJoin[0] != "1.1.1.1" {
		t.Fatalf("bad: %#v", config)
	}
	if config.StartJoin[1] != "2.2.2.2" {
		t.Fatalf("bad: %#v", config)
	}

	// UI Dir
	input = `{"ui_dir": "/opt/consul-ui"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.UiDir != "/opt/consul-ui" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestDecodeConfig_Service(t *testing.T) {
	// Basics
	input := `{"service": {"id": "red1", "name": "redis", "tags": ["master"], "port":8000, "check": {"script": "/bin/check_redis", "interval": "10s", "ttl": "15s" }}}`
	config, err := DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.Services) != 1 {
		t.Fatalf("missing service")
	}

	serv := config.Services[0]
	if serv.ID != "red1" {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Name != "redis" {
		t.Fatalf("bad: %v", serv)
	}

	if !strContains(serv.Tags, "master") {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Port != 8000 {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Check.Script != "/bin/check_redis" {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Check.Interval != 10*time.Second {
		t.Fatalf("bad: %v", serv)
	}

	if serv.Check.TTL != 15*time.Second {
		t.Fatalf("bad: %v", serv)
	}
}

func TestDecodeConfig_Check(t *testing.T) {
	// Basics
	input := `{"check": {"id": "chk1", "name": "mem", "notes": "foobar", "script": "/bin/check_redis", "interval": "10s", "ttl": "15s" }}`
	config, err := DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(config.Checks) != 1 {
		t.Fatalf("missing check")
	}

	chk := config.Checks[0]
	if chk.ID != "chk1" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Name != "mem" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Notes != "foobar" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Script != "/bin/check_redis" {
		t.Fatalf("bad: %v", chk)
	}

	if chk.Interval != 10*time.Second {
		t.Fatalf("bad: %v", chk)
	}

	if chk.TTL != 15*time.Second {
		t.Fatalf("bad: %v", chk)
	}
}

func TestMergeConfig(t *testing.T) {
	a := &Config{
		Bootstrap:      false,
		Datacenter:     "dc1",
		DataDir:        "/tmp/foo",
		DNSRecursor:    "127.0.0.1:1001",
		Domain:         "basic",
		LogLevel:       "debug",
		NodeName:       "foo",
		ClientAddr:     "127.0.0.1",
		BindAddr:       "127.0.0.1",
		AdvertiseAddr:  "127.0.0.1",
		Server:         false,
		LeaveOnTerm:    false,
		SkipLeaveOnInt: false,
		EnableDebug:    false,
	}

	b := &Config{
		Bootstrap:     true,
		Datacenter:    "dc2",
		DataDir:       "/tmp/bar",
		DNSRecursor:   "127.0.0.2:1001",
		Domain:        "other",
		LogLevel:      "info",
		NodeName:      "baz",
		ClientAddr:    "127.0.0.1",
		BindAddr:      "127.0.0.1",
		AdvertiseAddr: "127.0.0.1",
		Ports: PortConfig{
			DNS:     1,
			HTTP:    2,
			RPC:     3,
			SerfLan: 4,
			SerfWan: 5,
			Server:  6,
		},
		Server:         true,
		LeaveOnTerm:    true,
		SkipLeaveOnInt: true,
		EnableDebug:    true,
		VerifyIncoming: true,
		VerifyOutgoing: true,
		CAFile:         "test/ca.pem",
		CertFile:       "test/cert.pem",
		KeyFile:        "test/key.pem",
		Checks:         []*CheckDefinition{nil},
		Services:       []*ServiceDefinition{nil},
		StartJoin:      []string{"1.1.1.1"},
		UiDir:          "/opt/consul-ui",
	}

	c := MergeConfig(a, b)

	if !reflect.DeepEqual(c, b) {
		t.Fatalf("should be equal %v %v", c, b)
	}
}

func TestReadConfigPaths_badPath(t *testing.T) {
	_, err := ReadConfigPaths([]string{"/i/shouldnt/exist/ever/rainbows"})
	if err == nil {
		t.Fatal("should have err")
	}
}

func TestReadConfigPaths_file(t *testing.T) {
	tf, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	tf.Write([]byte(`{"node_name":"bar"}`))
	tf.Close()
	defer os.Remove(tf.Name())

	config, err := ReadConfigPaths([]string{tf.Name()})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "bar" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestReadConfigPaths_dir(t *testing.T) {
	td, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	err = ioutil.WriteFile(filepath.Join(td, "a.json"),
		[]byte(`{"node_name": "bar"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = ioutil.WriteFile(filepath.Join(td, "b.json"),
		[]byte(`{"node_name": "baz"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// A non-json file, shouldn't be read
	err = ioutil.WriteFile(filepath.Join(td, "c"),
		[]byte(`{"node_name": "bad"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	config, err := ReadConfigPaths([]string{td})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "baz" {
		t.Fatalf("bad: %#v", config)
	}
}
