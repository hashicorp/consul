package wanfed

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
)

const (
	// GossipPacketMaxIdleTime controls how long we keep an idle connection
	// open to a server.
	//
	// Conceptually similar to: agent/consul/server.go:serverRPCCache
	GossipPacketMaxIdleTime = 2 * time.Minute

	// GossipPacketMaxByteSize is the maximum allowed size of a packet
	// forwarded via wanfed. This is 4MB which should be way bigger than serf
	// or memberlist allow practically so it should never be hit in practice.
	GossipPacketMaxByteSize = 4 * 1024 * 1024
)

type MeshGatewayResolver func(datacenter string) string

type IngestionAwareTransport interface {
	memberlist.NodeAwareTransport
	IngestPacket(conn net.Conn, addr net.Addr, now time.Time, shouldClose bool) error
	IngestStream(conn net.Conn) error
}

func NewTransport(
	tlsConfigurator *tlsutil.Configurator,
	transport IngestionAwareTransport,
	datacenter string,
	gwResolver MeshGatewayResolver,
) (*Transport, error) {
	if tlsConfigurator == nil {
		return nil, errors.New("wanfed: tlsConfigurator is nil")
	}
	if gwResolver == nil {
		return nil, errors.New("wanfed: gwResolver is nil")
	}

	cp, err := newConnPool(GossipPacketMaxIdleTime)
	if err != nil {
		return nil, err
	}

	t := &Transport{
		IngestionAwareTransport: transport,
		tlsConfigurator:         tlsConfigurator,
		datacenter:              datacenter,
		gwResolver:              gwResolver,
		pool:                    cp,
	}
	return t, nil
}

type Transport struct {
	IngestionAwareTransport

	tlsConfigurator *tlsutil.Configurator
	datacenter      string
	gwResolver      MeshGatewayResolver
	pool            *connPool
}

var _ memberlist.NodeAwareTransport = (*Transport)(nil)

// Shutdown implements memberlist.Transport.
func (t *Transport) Shutdown() error {
	err1 := t.pool.Close()
	err2 := t.IngestionAwareTransport.Shutdown()
	if err2 != nil {
		// the more important error is err2
		return err2
	}
	if err1 != nil {
		return err1
	}
	return nil
}

// WriteToAddress implements memberlist.NodeAwareTransport.
func (t *Transport) WriteToAddress(b []byte, addr memberlist.Address) (time.Time, error) {
	node, dc, err := SplitNodeName(addr.Name)
	if err != nil {
		return time.Time{}, err
	}

	if dc != t.datacenter {
		gwAddr := t.gwResolver(dc)
		if gwAddr == "" {
			return time.Time{}, structs.ErrDCNotAvailable
		}

		dialFunc := func() (net.Conn, error) {
			return t.dial(dc, node, pool.ALPN_WANGossipPacket, gwAddr)
		}
		conn, err := t.pool.AcquireOrDial(addr.Name, dialFunc)
		if err != nil {
			return time.Time{}, err
		}
		defer conn.ReturnOrClose()

		// Send the length first.
		if err := binary.Write(conn, binary.BigEndian, uint32(len(b))); err != nil {
			conn.MarkFailed()
			return time.Time{}, err
		}

		if _, err = conn.Write(b); err != nil {
			conn.MarkFailed()
			return time.Time{}, err
		}

		return time.Now(), nil
	}

	return t.IngestionAwareTransport.WriteToAddress(b, addr)
}

// DialAddressTimeout implements memberlist.NodeAwareTransport.
func (t *Transport) DialAddressTimeout(addr memberlist.Address, timeout time.Duration) (net.Conn, error) {
	node, dc, err := SplitNodeName(addr.Name)
	if err != nil {
		return nil, err
	}

	if dc != t.datacenter {
		gwAddr := t.gwResolver(dc)
		if gwAddr == "" {
			return nil, structs.ErrDCNotAvailable
		}

		return t.dial(dc, node, pool.ALPN_WANGossipStream, gwAddr)
	}

	return t.IngestionAwareTransport.DialAddressTimeout(addr, timeout)
}

// NOTE: There is a close mirror of this method in agent/pool/pool.go:DialTimeoutWithRPCType
func (t *Transport) dial(dc, nodeName, nextProto, addr string) (net.Conn, error) {
	wrapper := t.tlsConfigurator.OutgoingALPNRPCWrapper()
	if wrapper == nil {
		return nil, fmt.Errorf("wanfed: cannot dial via a mesh gateway when outgoing TLS is disabled")
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}

	rawConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	if tcp, ok := rawConn.(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetNoDelay(true)
	}

	tlsConn, err := wrapper(dc, nodeName, nextProto, rawConn)
	if err != nil {
		return nil, err
	}

	return tlsConn, nil
}

// SplitNodeName splits a node name as it would be represented in
// serf/memberlist in the WAN pool of the form "<short-node-name>.<datacenter>"
// like "nyc-web42.dc5" => "nyc-web42" & "dc5"
func SplitNodeName(nodeName string) (shortName, dc string, err error) {
	parts := strings.Split(nodeName, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("node name does not encode a datacenter: %s", nodeName)
	}
	return parts[0], parts[1], nil
}
