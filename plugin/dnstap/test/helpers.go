package test

import (
	"net"
	"reflect"

	"github.com/coredns/coredns/plugin/dnstap/msg"

	tap "github.com/dnstap/golang-dnstap"
	"golang.org/x/net/context"
)

// Context is a message trap.
type Context struct {
	context.Context
	TrapTapper
}

// TestingData returns the Data matching coredns/test.ResponseWriter.
func TestingData() (d *msg.Data) {
	d = &msg.Data{
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

// TapMessage adds the message to the trap.
func (t *TrapTapper) TapMessage(m *tap.Message) error {
	t.Trap = append(t.Trap, m)
	return nil
}

// TapBuilder returns a test msg.Builder.
func (t *TrapTapper) TapBuilder() msg.Builder {
	return msg.Builder{Full: t.Full}
}
