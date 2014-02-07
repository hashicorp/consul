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
	input = `{"dns_addr": "127.0.0.1:8500", "recursor": "8.8.8.8", "domain": "foobar"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.DNSAddr != "127.0.0.1:8500" {
		t.Fatalf("bad: %#v", config)
	}

	if config.DNSRecursor != "8.8.8.8" {
		t.Fatalf("bad: %#v", config)
	}

	if config.Domain != "foobar" {
		t.Fatalf("bad: %#v", config)
	}

	// RPC configs
	input = `{"http_addr": "127.0.0.1:1234", "rpc_addr": "127.0.0.1:8100"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.HTTPAddr != "127.0.0.1:1234" {
		t.Fatalf("bad: %#v", config)
	}

	if config.RPCAddr != "127.0.0.1:8100" {
		t.Fatalf("bad: %#v", config)
	}

	// Serf configs
	input = `{"serf_bind_addr": "127.0.0.2", "serf_lan_port": 1000, "serf_wan_port": 2000}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.SerfBindAddr != "127.0.0.2" {
		t.Fatalf("bad: %#v", config)
	}

	if config.SerfLanPort != 1000 {
		t.Fatalf("bad: %#v", config)
	}

	if config.SerfWanPort != 2000 {
		t.Fatalf("bad: %#v", config)
	}

	// Server addrs
	input = `{"server_addr": "127.0.0.1:8000", "advertise_addr": "127.0.0.1:8000"}`
	config, err = DecodeConfig(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.ServerAddr != "127.0.0.1:8000" {
		t.Fatalf("bad: %#v", config)
	}

	if config.AdvertiseAddr != "127.0.0.1:8000" {
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
}

func TestDecodeConfig_Service(t *testing.T) {
	// Basics
	input := `{"service": {"id": "red1", "name": "redis", "tag": "master", "port":8000, "check": {"script": "/bin/check_redis", "interval": "10s", "ttl": "15s" }}}`
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

	if serv.Tag != "master" {
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
		DNSAddr:        "127.0.0.1:1000",
		DNSRecursor:    "127.0.0.1:1001",
		Domain:         "basic",
		HTTPAddr:       "",
		LogLevel:       "debug",
		NodeName:       "foo",
		RPCAddr:        "",
		SerfBindAddr:   "127.0.0.1",
		SerfLanPort:    1000,
		SerfWanPort:    2000,
		ServerAddr:     "127.0.0.1:8000",
		AdvertiseAddr:  "127.0.0.1:8000",
		Server:         false,
		LeaveOnTerm:    false,
		SkipLeaveOnInt: false,
	}

	b := &Config{
		Bootstrap:      true,
		Datacenter:     "dc2",
		DataDir:        "/tmp/bar",
		DNSAddr:        "127.0.0.2:1000",
		DNSRecursor:    "127.0.0.2:1001",
		Domain:         "other",
		HTTPAddr:       "127.0.0.1:12345",
		LogLevel:       "info",
		NodeName:       "baz",
		RPCAddr:        "127.0.0.1:9999",
		SerfBindAddr:   "127.0.0.2",
		SerfLanPort:    3000,
		SerfWanPort:    4000,
		ServerAddr:     "127.0.0.2:8000",
		AdvertiseAddr:  "127.0.0.2:8000",
		Server:         true,
		LeaveOnTerm:    true,
		SkipLeaveOnInt: true,
		Checks:         []*CheckDefinition{nil},
		Services:       []*ServiceDefinition{nil},
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
