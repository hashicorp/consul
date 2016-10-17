package response

import (
	"testing"

	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestTypifyNilMsg(t *testing.T) {
	var m *dns.Msg

	ty, _ := Typify(m)
	if ty != OtherError {
		t.Errorf("message wrongly typified, expected OtherError, got %d", ty)
	}
}

func TestClassifyDelegation(t *testing.T) {
	m := delegationMsg()
	mt, _ := Typify(m)
	if mt != Delegation {
		t.Errorf("message is wrongly classified, expected delegation, got %d", mt)
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
