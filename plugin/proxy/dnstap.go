package proxy

import (
	"time"

	"github.com/coredns/coredns/plugin/dnstap"
	"github.com/coredns/coredns/request"

	tap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func toDnstap(ctx context.Context, host string, ex Exchanger, state request.Request, reply *dns.Msg, start time.Time) error {
	tapper := dnstap.TapperFromContext(ctx)
	if tapper == nil {
		return nil
	}

	// Query
	b := tapper.TapBuilder()
	b.TimeSec = uint64(start.Unix())
	if err := b.HostPort(host); err != nil {
		return err
	}

	t := ex.Transport()
	if t == "" {
		t = state.Proto()
	}
	if t == "tcp" {
		b.SocketProto = tap.SocketProtocol_TCP
	} else {
		b.SocketProto = tap.SocketProtocol_UDP
	}

	if err := b.Msg(state.Req); err != nil {
		return err
	}

	if err := tapper.TapMessage(b.ToOutsideQuery(tap.Message_FORWARDER_QUERY)); err != nil {
		return err
	}

	// Response
	if reply != nil {
		b.TimeSec = uint64(time.Now().Unix())
		if err := b.Msg(reply); err != nil {
			return err
		}
		return tapper.TapMessage(b.ToOutsideResponse(tap.Message_FORWARDER_RESPONSE))
	}
	return nil
}
