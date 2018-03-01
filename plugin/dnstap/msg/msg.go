package msg

import (
	"errors"
	"net"
	"strconv"
	"time"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
)

// Builder helps to build a Dnstap message.
type Builder struct {
	Packed      []byte
	SocketProto tap.SocketProtocol
	SocketFam   tap.SocketFamily
	Address     net.IP
	Port        uint32
	TimeSec     uint64

	err error
}

// New returns a new Builder
func New() *Builder {
	return &Builder{}
}

// Addr adds the remote address to the message.
func (b *Builder) Addr(remote net.Addr) *Builder {
	if b.err != nil {
		return b
	}

	switch addr := remote.(type) {
	case *net.TCPAddr:
		b.Address = addr.IP
		b.Port = uint32(addr.Port)
		b.SocketProto = tap.SocketProtocol_TCP
	case *net.UDPAddr:
		b.Address = addr.IP
		b.Port = uint32(addr.Port)
		b.SocketProto = tap.SocketProtocol_UDP
	default:
		b.err = errors.New("unknown remote address type")
		return b
	}

	if b.Address.To4() != nil {
		b.SocketFam = tap.SocketFamily_INET
	} else {
		b.SocketFam = tap.SocketFamily_INET6
	}
	return b
}

// Msg adds the raw DNS message to the dnstap message.
func (b *Builder) Msg(m *dns.Msg) *Builder {
	if b.err != nil {
		return b
	}

	b.Packed, b.err = m.Pack()
	return b
}

// HostPort adds the remote address as encoded by dnsutil.ParseHostPortOrFile to the message.
func (b *Builder) HostPort(addr string) *Builder {
	ip, port, err := net.SplitHostPort(addr)
	if err != nil {
		b.err = err
		return b
	}
	p, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		b.err = err
		return b
	}
	b.Port = uint32(p)

	if ip := net.ParseIP(ip); ip != nil {
		b.Address = []byte(ip)
		if ip := ip.To4(); ip != nil {
			b.SocketFam = tap.SocketFamily_INET
		} else {
			b.SocketFam = tap.SocketFamily_INET6
		}
		return b
	}
	b.err = errors.New("not an ip address")
	return b
}

// Time adds the timestamp to the message.
func (b *Builder) Time(ts time.Time) *Builder {
	b.TimeSec = uint64(ts.Unix())
	return b
}

// ToClientResponse transforms Data into a client response message.
func (b *Builder) ToClientResponse() (*tap.Message, error) {
	t := tap.Message_CLIENT_RESPONSE
	return &tap.Message{
		Type:            &t,
		SocketFamily:    &b.SocketFam,
		SocketProtocol:  &b.SocketProto,
		ResponseTimeSec: &b.TimeSec,
		ResponseMessage: b.Packed,
		QueryAddress:    b.Address,
		QueryPort:       &b.Port,
	}, b.err
}

// ToClientQuery transforms Data into a client query message.
func (b *Builder) ToClientQuery() (*tap.Message, error) {
	t := tap.Message_CLIENT_QUERY
	return &tap.Message{
		Type:           &t,
		SocketFamily:   &b.SocketFam,
		SocketProtocol: &b.SocketProto,
		QueryTimeSec:   &b.TimeSec,
		QueryMessage:   b.Packed,
		QueryAddress:   b.Address,
		QueryPort:      &b.Port,
	}, b.err
}

// ToOutsideQuery transforms the data into a forwarder or resolver query message.
func (b *Builder) ToOutsideQuery(t tap.Message_Type) (*tap.Message, error) {
	return &tap.Message{
		Type:            &t,
		SocketFamily:    &b.SocketFam,
		SocketProtocol:  &b.SocketProto,
		QueryTimeSec:    &b.TimeSec,
		QueryMessage:    b.Packed,
		ResponseAddress: b.Address,
		ResponsePort:    &b.Port,
	}, b.err
}

// ToOutsideResponse transforms the data into a forwarder or resolver response message.
func (b *Builder) ToOutsideResponse(t tap.Message_Type) (*tap.Message, error) {
	return &tap.Message{
		Type:            &t,
		SocketFamily:    &b.SocketFam,
		SocketProtocol:  &b.SocketProto,
		ResponseTimeSec: &b.TimeSec,
		ResponseMessage: b.Packed,
		ResponseAddress: b.Address,
		ResponsePort:    &b.Port,
	}, b.err
}
