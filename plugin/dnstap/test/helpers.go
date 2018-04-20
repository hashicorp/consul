package test

import (
	"net"
	"reflect"

	"github.com/coredns/coredns/plugin/dnstap/msg"

	"context"

	tap "github.com/dnstap/golang-dnstap"
)

// Context is a message trap.
type Context struct {
	context.Context
	TrapTapper
}

// TestingData returns the Data matching coredns/test.ResponseWriter.
func TestingData() (d *msg.Builder) {
	d = &msg.Builder{
		SocketFam:   tap.SocketFamily_INET,
		SocketProto: tap.SocketProtocol_UDP,
		Address:     net.ParseIP("10.240.0.1"),
		Port:        40212,
	}
	return
}

type comp struct {
	Type  *tap.Message_Type
	SF    *tap.SocketFamily
	SP    *tap.SocketProtocol
	QA    []byte
	RA    []byte
	QP    *uint32
	RP    *uint32
	QTSec bool
	RTSec bool
	RM    []byte
	QM    []byte
}

func toComp(m *tap.Message) comp {
	return comp{
		Type:  m.Type,
		SF:    m.SocketFamily,
		SP:    m.SocketProtocol,
		QA:    m.QueryAddress,
		RA:    m.ResponseAddress,
		QP:    m.QueryPort,
		RP:    m.ResponsePort,
		QTSec: m.QueryTimeSec != nil,
		RTSec: m.ResponseTimeSec != nil,
		RM:    m.ResponseMessage,
		QM:    m.QueryMessage,
	}
}

// MsgEqual compares two dnstap messages ignoring timestamps.
func MsgEqual(a, b *tap.Message) bool {
	return reflect.DeepEqual(toComp(a), toComp(b))
}

// TrapTapper traps messages.
type TrapTapper struct {
	Trap []*tap.Message
	Full bool
}

// Pack returns field Full.
func (t *TrapTapper) Pack() bool {
	return t.Full
}

// TapMessage adds the message to the trap.
func (t *TrapTapper) TapMessage(m *tap.Message) {
	t.Trap = append(t.Trap, m)
}
