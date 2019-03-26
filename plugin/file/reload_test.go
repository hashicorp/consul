package file

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestZoneReload(t *testing.T) {
	fileName, rm, err := test.TempFile(".", reloadZoneTest)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()
	reader, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("Failed to open zone: %s", err)
	}
	z, err := Parse(reader, "miek.nl", fileName, 0)
	if err != nil {
		t.Fatalf("Failed to parse zone: %s", err)
	}

	TickTime = 500 * time.Millisecond
	z.ReloadInterval = 500 * time.Millisecond
	z.Reload()
	time.Sleep(time.Second)

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("miek.nl", dns.TypeSOA)
	state := request.Request{W: &test.ResponseWriter{}, Req: r}
	if _, _, _, res := z.Lookup(ctx, state, "miek.nl."); res != Success {
		t.Fatalf("Failed to lookup, got %d", res)
	}

	r = new(dns.Msg)
	r.SetQuestion("miek.nl", dns.TypeNS)
	state = request.Request{W: &test.ResponseWriter{}, Req: r}
	if _, _, _, res := z.Lookup(ctx, state, "miek.nl."); res != Success {
		t.Fatalf("Failed to lookup, got %d", res)
	}

	if len(z.All()) != 5 {
		t.Fatalf("Expected 5 RRs, got %d", len(z.All()))
	}
	if err := ioutil.WriteFile(fileName, []byte(reloadZone2Test), 0644); err != nil {
		t.Fatalf("Failed to write new zone data: %s", err)
	}
	// Could still be racy, but we need to wait a bit for the event to be seen
	time.Sleep(1 * time.Second)

	if len(z.All()) != 3 {
		t.Fatalf("Expected 3 RRs, got %d", len(z.All()))
	}
}

func TestZoneReloadSOAChange(t *testing.T) {
	_, err := Parse(strings.NewReader(reloadZoneTest), "miek.nl.", "stdin", 1460175181)
	if err == nil {
		t.Fatalf("Zone should not have been re-parsed")
	}

}

const reloadZoneTest = `miek.nl.		1627	IN	SOA	linode.atoom.net. miek.miek.nl. 1460175181 14400 3600 604800 14400
miek.nl.		1627	IN	NS	ext.ns.whyscream.net.
miek.nl.		1627	IN	NS	omval.tednet.nl.
miek.nl.		1627	IN	NS	linode.atoom.net.
miek.nl.		1627	IN	NS	ns-ext.nlnetlabs.nl.
`

const reloadZone2Test = `miek.nl.		1627	IN	SOA	linode.atoom.net. miek.miek.nl. 1460175182 14400 3600 604800 14400
miek.nl.		1627	IN	NS	ext.ns.whyscream.net.
miek.nl.		1627	IN	NS	omval.tednet.nl.
`
