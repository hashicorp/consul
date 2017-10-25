// Package msg helps to build a dnstap Message.
package msg

import (
	"errors"
	"net"
	"strconv"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

// Builder helps to build Data by being aware of the dnstap plugin configuration.
type Builder struct {
	Full bool
	Data
}

// AddrMsg parses the info of net.Addr and dns.Msg.
func (b *Builder) AddrMsg(a net.Addr, m *dns.Msg) (err error) {
	err = b.RemoteAddr(a)
	if err != nil {
		return
	}
	return b.Msg(m)
}

// Msg parses the info of dns.Msg.
func (b *Builder) Msg(m *dns.Msg) (err error) {
	if b.Full {
		err = b.Pack(m)
	}
	return
}

// Data helps to build a dnstap Message.
// It can be transformed into the actual Message using this package.
type Data struct {
	Packed      []byte
	SocketProto tap.SocketProtocol
	SocketFam   tap.SocketFamily
	Address     []byte
	Port        uint32
	TimeSec     uint64
}

// HostPort decodes into Data any string returned by dnsutil.ParseHostPortOrFile.
func (d *Data) HostPort(addr string) error {
	ip, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	p, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return err
	}
	d.Port = uint32(p)

	if ip := net.ParseIP(ip); ip != nil {
		d.Address = []byte(ip)
		if ip := ip.To4(); ip != nil {
			d.SocketFam = tap.SocketFamily_INET
		} else {
			d.SocketFam = tap.SocketFamily_INET6
		}
		return nil
	}
	return errors.New("not an ip address")
}

// RemoteAddr parses the information about the remote address into Data.
func (d *Data) RemoteAddr(remote net.Addr) error {
	switch addr := remote.(type) {
	case *net.TCPAddr:
		d.Address = addr.IP
		d.Port = uint32(addr.Port)
		d.SocketProto = tap.SocketProtocol_TCP
	case *net.UDPAddr:
		d.Address = addr.IP
		d.Port = uint32(addr.Port)
		d.SocketProto = tap.SocketProtocol_UDP
	default:
		return errors.New("unknown remote address type")
	}

	if a := net.IP(d.Address); a.To4() != nil {
		d.SocketFam = tap.SocketFamily_INET
	} else {
		d.SocketFam = tap.SocketFamily_INET6
	}

	return nil
}

// Pack encodes the DNS message into Data.
func (d *Data) Pack(m *dns.Msg) error {
	packed, err := m.Pack()
	if err != nil {
		return err
	}
	d.Packed = packed
	return nil
}

// ToClientResponse transforms Data into a client response message.
func (d *Data) ToClientResponse() *tap.Message {
	t := tap.Message_CLIENT_RESPONSE
	return &tap.Message{
		Type:            &t,
		SocketFamily:    &d.SocketFam,
		SocketProtocol:  &d.SocketProto,
		ResponseTimeSec: &d.TimeSec,
		ResponseMessage: d.Packed,
		QueryAddress:    d.Address,
		QueryPort:       &d.Port,
	}
}

// ToClientQuery transforms Data into a client query message.
func (d *Data) ToClientQuery() *tap.Message {
	t := tap.Message_CLIENT_QUERY
	return &tap.Message{
		Type:           &t,
		SocketFamily:   &d.SocketFam,
		SocketProtocol: &d.SocketProto,
		QueryTimeSec:   &d.TimeSec,
		QueryMessage:   d.Packed,
		QueryAddress:   d.Address,
		QueryPort:      &d.Port,
	}
}

// ToOutsideQuery transforms the data into a forwarder or resolver query message.
func (d *Data) ToOutsideQuery(t tap.Message_Type) *tap.Message {
	return &tap.Message{
		Type:            &t,
		SocketFamily:    &d.SocketFam,
		SocketProtocol:  &d.SocketProto,
		QueryTimeSec:    &d.TimeSec,
		QueryMessage:    d.Packed,
		ResponseAddress: d.Address,
		ResponsePort:    &d.Port,
	}
}

// ToOutsideResponse transforms the data into a forwarder or resolver response message.
func (d *Data) ToOutsideResponse(t tap.Message_Type) *tap.Message {
	return &tap.Message{
		Type:            &t,
		SocketFamily:    &d.SocketFam,
		SocketProtocol:  &d.SocketProto,
		ResponseTimeSec: &d.TimeSec,
		ResponseMessage: d.Packed,
		ResponseAddress: d.Address,
		ResponsePort:    &d.Port,
	}
}
