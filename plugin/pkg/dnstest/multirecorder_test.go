package dnstest

import (
	"testing"

	"github.com/miekg/dns"
)

func TestMultiWriteMsg(t *testing.T) {
	w := &responseWriter{}
	record := NewMultiRecorder(w)

	responseTestName := "testmsg.example.org."
	responseTestMsg := new(dns.Msg)
	responseTestMsg.SetQuestion(responseTestName, dns.TypeA)

	record.WriteMsg(responseTestMsg)
	record.WriteMsg(responseTestMsg)

	if len(record.Msgs) != 2 {
		t.Fatalf("Expected 2 messages to be written, but instead found %d\n", len(record.Msgs))

	}
	if record.Len != responseTestMsg.Len()*2 {
		t.Fatalf("Expected the bytes written counter to be %d, but instead found %d\n", responseTestMsg.Len()*2, record.Len)
	}
}

func TestMultiWrite(t *testing.T) {
	w := &responseWriter{}
	record := NewRecorder(w)
	responseTest := []byte("testmsg.example.org.")

	record.Write(responseTest)
	record.Write(responseTest)
	if record.Len != len(responseTest)*2 {
		t.Fatalf("Expected the bytes written counter to be %d, but instead found %d\n", len(responseTest)*2, record.Len)
	}
}
