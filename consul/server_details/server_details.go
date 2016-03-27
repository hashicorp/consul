package server_details

import (
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/serf/serf"
)

// Key is used in maps and for equality tests.  A key is based on endpoints.
type Key struct {
	Datacenter string
	Port       int
	AddrString string
}

// Equal compares two Key objects
func (k *Key) Equal(x *Key) bool {
	return k.Datacenter == x.Datacenter &&
		k.Port == x.Port &&
		k.AddrString == x.AddrString
}

// ServerDetails is used to return details of a consul server
type ServerDetails struct {
	Name       string
	Datacenter string
	Port       int
	Bootstrap  bool
	Expect     int
	Version    int
	Addr       net.Addr
}

// Key returns the corresponding Key
func (s *ServerDetails) Key() *Key {
	return &Key{
		Datacenter: s.Datacenter,
		Port:       s.Port,
		AddrString: s.Addr.String() + s.Addr.Network(),
	}
}

// String returns a string representation of ServerDetails
func (s *ServerDetails) String() string {
	return fmt.Sprintf("%s (Addr: %s) (DC: %s)", s.Name, s.Addr, s.Datacenter)
}

// IsConsulServer returns true if a serf member is a consul server. Returns a
// bool and a pointer to the ServerDetails.
func IsConsulServer(m serf.Member) (bool, *ServerDetails) {
	if m.Tags["role"] != "consul" {
		return false, nil
	}

	datacenter := m.Tags["dc"]
	_, bootstrap := m.Tags["bootstrap"]

	expect := 0
	expect_str, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expect_str)
		if err != nil {
			return false, nil
		}
	}

	port_str := m.Tags["port"]
	port, err := strconv.Atoi(port_str)
	if err != nil {
		return false, nil
	}

	vsn_str := m.Tags["vsn"]
	vsn, err := strconv.Atoi(vsn_str)
	if err != nil {
		return false, nil
	}

	addr := &net.TCPAddr{IP: m.Addr, Port: port}

	parts := &ServerDetails{
		Name:       m.Name,
		Datacenter: datacenter,
		Port:       port,
		Bootstrap:  bootstrap,
		Expect:     expect,
		Addr:       addr,
		Version:    vsn,
	}
	return true, parts
}
