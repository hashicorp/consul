package response

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestTypifyNilMsg(t *testing.T) {
	var m *dns.Msg

	ty, _ := Typify(m, time.Now().UTC())
	if ty != OtherError {
		t.Errorf("message wrongly typified, expected OtherError, got %s", ty)
	}
}

func TestTypifyDelegation(t *testing.T) {
	m := delegationMsg()
	mt, _ := Typify(m, time.Now().UTC())
	if mt != Delegation {
		t.Errorf("message is wrongly typified, expected Delegation, got %s", mt)
	}
}

func TestTypifyRRSIG(t *testing.T) {
	now, _ := time.Parse(time.UnixDate, "Fri Apr 21 10:51:21 BST 2017")
	utc := now.UTC()

	m := delegationMsgRRSIGOK()
	if mt, _ := Typify(m, utc); mt != Delegation {
		t.Errorf("message is wrongly typified, expected Delegation, got %s", mt)
	}

	// Still a Delegation because EDNS0 OPT DO bool is not set, so we won't check the sigs.
	m = delegationMsgRRSIGFail()
	if mt, _ := Typify(m, utc); mt != Delegation {
		t.Errorf("message is wrongly typified, expected Delegation, got %s", mt)
	}

	m = delegationMsgRRSIGFail()
	m = addOpt(m)
	if mt, _ := Typify(m, utc); mt != OtherError {
		t.Errorf("message is wrongly typified, expected OtherError, got %s", mt)
	}
}

func delegationMsg() *dns.Msg {
	return &dns.Msg{
		Ns: []dns.RR{
			test.NS("miek.nl.	3600	IN	NS	linode.atoom.net."),
			test.NS("miek.nl.	3600	IN	NS	ns-ext.nlnetlabs.nl."),
			test.NS("miek.nl.	3600	IN	NS	omval.tednet.nl."),
		},
		Extra: []dns.RR{
			test.A("omval.tednet.nl.	3600	IN	A	185.49.141.42"),
			test.AAAA("omval.tednet.nl.	3600	IN	AAAA	2a04:b900:0:100::42"),
		},
	}
}

func delegationMsgRRSIGOK() *dns.Msg {
	del := delegationMsg()
	del.Ns = append(del.Ns,
		test.RRSIG("miek.nl.		1800	IN	RRSIG	NS 8 2 1800 20170521031301 20170421031301 12051 miek.nl. PIUu3TKX/sB/N1n1E1yWxHHIcPnc2q6Wq9InShk+5ptRqChqKdZNMLDm gCq+1bQAZ7jGvn2PbwTwE65JzES7T+hEiqR5PU23DsidvZyClbZ9l0xG JtKwgzGXLtUHxp4xv/Plq+rq/7pOG61bNCxRyS7WS7i7QcCCWT1BCcv+ wZ0="),
	)
	return del
}

func delegationMsgRRSIGFail() *dns.Msg {
	del := delegationMsg()
	del.Ns = append(del.Ns,
		test.RRSIG("miek.nl.		1800	IN	RRSIG	NS 8 2 1800 20160521031301 20160421031301 12051 miek.nl. PIUu3TKX/sB/N1n1E1yWxHHIcPnc2q6Wq9InShk+5ptRqChqKdZNMLDm gCq+1bQAZ7jGvn2PbwTwE65JzES7T+hEiqR5PU23DsidvZyClbZ9l0xG JtKwgzGXLtUHxp4xv/Plq+rq/7pOG61bNCxRyS7WS7i7QcCCWT1BCcv+ wZ0="),
	)
	return del
}

func addOpt(m *dns.Msg) *dns.Msg {
	m.Extra = append(m.Extra, test.OPT(4096, true))
	return m
}
