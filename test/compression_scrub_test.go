package test

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

func TestCompressScrub(t *testing.T) {
	corefile := `example.org:0 {
                       erratic {
			  drop 0
			  delay 0
			  large
		       }
		     }`

	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	c, err := net.Dial("udp", udp)
	if err != nil {
		t.Fatalf("Could not dial %s", err)
	}
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	q, _ := m.Pack()

	c.Write(q)
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil || n == 0 {
		t.Errorf("Expected reply, got: %s", err)
		return
	}
	if n >= 512 {
		t.Fatalf("Expected returned packet to be < 512, got %d", n)
	}
	buf = buf[:n]
	// If there is compression in the returned packet we should look for compression pointers, if found
	// the pointers should return to the domain name in the query (the first domain name that's available for
	// compression. This means we're looking for a combo where the pointers is detected and the offset is 12
	// the position of the first name after the header. The erratic plugin adds 30 RRs that should all be compressed.
	found := 0
	for i := 0; i < len(buf)-1; i++ {
		if buf[i]&0xC0 == 0xC0 {
			off := (int(buf[i])^0xC0)<<8 | int(buf[i+1])
			if off == 12 {
				found++
			}
		}
	}
	if found != 30 {
		t.Errorf("Failed to find all compression pointers in the packet, wanted 30, got %d", found)
	}
}
