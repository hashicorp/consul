package dnstest

import (
	"testing"

	"github.com/miekg/dns"
)

type responseWriter struct{ dns.ResponseWriter }

func (r *responseWriter) WriteMsg(m *dns.Msg) error     { return nil }
func (r *responseWriter) Write(buf []byte) (int, error) { return len(buf), nil }

func TestNewRecorder(t *testing.T) {
	w := &responseWriter{}
	record := NewRecorder(w)
	if record.ResponseWriter != w {
		t.Fatalf("Expected Response writer in the Recording to be same as the one sent\n")
	}
	if record.Rcode != dns.RcodeSuccess {
		t.Fatalf("Expected recorded status to be dns.RcodeSuccess (%d) , but found %d\n ", dns.RcodeSuccess, record.Rcode)
	}
}

func TestWriteMsg(t *testing.T) {
	w := &responseWriter{}
	record := NewRecorder(w)
	responseTestName := "testmsg.example.org."
	responseTestMsg := new(dns.Msg)
	responseTestMsg.SetQuestion(responseTestName, dns.TypeA)

	record.WriteMsg(responseTestMsg)
	if record.Len != responseTestMsg.Len() {
		t.Fatalf("Expected the bytes written counter to be %d, but instead found %d\n", responseTestMsg.Len(), record.Len)
	}
	if x := record.Msg.Question[0].Name; x != responseTestName {
		t.Fatalf("Expected Msg Qname to be %s , but found %s\n", responseTestName, x)
	}
}

func TestWrite(t *testing.T) {
	w := &responseWriter{}
	record := NewRecorder(w)
	responseTest := []byte("testmsg.example.org.")

	record.Write(responseTest)
	if record.Len != len(responseTest) {
		t.Fatalf("Expected the bytes written counter to be %d, but instead found %d\n", len(responseTest), record.Len)
	}
}
