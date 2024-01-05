// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package wanfed

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/hashicorp/consul/agent/pool"
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
		dialFunc := func() (net.Conn, error) {
			return t.dial(dc, node, pool.ALPN_WANGossipPacket)
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
		return t.dial(dc, node, pool.ALPN_WANGossipStream)
	}

	return t.IngestionAwareTransport.DialAddressTimeout(addr, timeout)
}

func (t *Transport) dial(dc, nodeName, nextProto string) (net.Conn, error) {
	conn, _, err := pool.DialRPCViaMeshGateway(
		context.Background(),
		dc,
		nodeName,
		nil, // TODO(rb): thread source address through here?
		t.tlsConfigurator.OutgoingALPNRPCWrapper(),
		nextProto,
		true,
		t.gwResolver,
	)
	return conn, err
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
