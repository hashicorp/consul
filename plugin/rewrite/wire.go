package rewrite

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// ipToWire writes IP address to wire/binary format, 4 or 16 bytes depends on IPV4 or IPV6.
func ipToWire(family int, ipAddr string) ([]byte, error) {

	switch family {
	case 1:
		return net.ParseIP(ipAddr).To4(), nil
	case 2:
		return net.ParseIP(ipAddr).To16(), nil
	}
	return nil, fmt.Errorf("invalid IP address family (i.e. version) %d", family)
}

// uint16ToWire writes unit16 to wire/binary format
func uint16ToWire(data uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(data))
	return buf
}

// portToWire writes port to wire/binary format, 2 bytes
func portToWire(portStr string) ([]byte, error) {

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	return uint16ToWire(uint16(port)), nil
}
