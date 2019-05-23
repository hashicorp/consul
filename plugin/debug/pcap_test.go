package debug

import (
	"bytes"
	"fmt"
	golog "log"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/log"

	"github.com/miekg/dns"
)

func msg() *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion("example.local.", dns.TypeA)
	m.SetEdns0(4096, true)
	m.Id = 10
	return m
}

func TestNoDebug(t *testing.T) {
	// Must come first, because set log.D.Set() which is impossible to undo.
	var f bytes.Buffer
	golog.SetOutput(&f)

	str := "Hi There!"
	Hexdumpf(msg(), "%s %d", str, 10)
	if len(f.Bytes()) != 0 {
		t.Errorf("Expected no output, got %d bytes", len(f.Bytes()))
	}
}

func ExampleLogHexdump() {
	buf, _ := msg().Pack()
	h := hexdump(buf)
	fmt.Println(string(h))

	// Output:
	// debug: 000000 00 0a 01 00 00 01 00 00 00 00 00 01 07 65 78 61
	// debug: 000010 6d 70 6c 65 05 6c 6f 63 61 6c 00 00 01 00 01 00
	// debug: 000020 00 29 10 00 00 00 80 00 00 00
	// debug: 00002a
}

func TestHexdump(t *testing.T) {
	var f bytes.Buffer
	golog.SetOutput(&f)
	log.D.Set()

	str := "Hi There!"
	Hexdump(msg(), str)
	logged := f.String()

	if !strings.Contains(logged, "[DEBUG] "+str) {
		t.Errorf("The string %s, is not contained in the logged output: %s", str, logged)
	}
}

func TestHexdumpf(t *testing.T) {
	var f bytes.Buffer
	golog.SetOutput(&f)
	log.D.Set()

	str := "Hi There!"
	Hexdumpf(msg(), "%s %d", str, 10)
	logged := f.String()

	if !strings.Contains(logged, "[DEBUG] "+fmt.Sprintf("%s %d", str, 10)) {
		t.Errorf("The string %s %d, is not contained in the logged output: %s", str, 10, logged)
	}
}
