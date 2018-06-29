package variables

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

const (
	queryName  = "qname"
	queryType  = "qtype"
	clientIP   = "client_ip"
	clientPort = "client_port"
	protocol   = "protocol"
	serverIP   = "server_ip"
	serverPort = "server_port"
)

// All is a list of available variables provided by GetMetadataValue
var All = []string{queryName, queryType, clientIP, clientPort, protocol, serverIP, serverPort}

// GetValue calculates and returns the data specified by the variable name.
// Supported varNames are listed in allProvidedVars.
func GetValue(varName string, w dns.ResponseWriter, r *dns.Msg) ([]byte, error) {
	req := request.Request{W: w, Req: r}
	switch varName {
	case queryName:
		//Query name is written as ascii string
		return []byte(req.QName()), nil

	case queryType:
		return uint16ToWire(req.QType()), nil

	case clientIP:
		return ipToWire(req.Family(), req.IP())

	case clientPort:
		return portToWire(req.Port())

	case protocol:
		// Proto is written as ascii string
		return []byte(req.Proto()), nil

	case serverIP:
		ip, _, err := net.SplitHostPort(w.LocalAddr().String())
		if err != nil {
			ip = w.RemoteAddr().String()
		}
		return ipToWire(family(w.RemoteAddr()), ip)

	case serverPort:
		_, port, err := net.SplitHostPort(w.LocalAddr().String())
		if err != nil {
			port = "0"
		}
		return portToWire(port)
	}

	return nil, fmt.Errorf("unable to extract data for variable %s", varName)
}

// uint16ToWire writes unit16 to wire/binary format
func uint16ToWire(data uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(data))
	return buf
}

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

// portToWire writes port to wire/binary format, 2 bytes
func portToWire(portStr string) ([]byte, error) {

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	return uint16ToWire(uint16(port)), nil
}

// Family returns the family of the transport, 1 for IPv4 and 2 for IPv6.
func family(ip net.Addr) int {
	var a net.IP
	if i, ok := ip.(*net.UDPAddr); ok {
		a = i.IP
	}
	if i, ok := ip.(*net.TCPAddr); ok {
		a = i.IP
	}
	if a.To4() != nil {
		return 1
	}
	return 2
}
