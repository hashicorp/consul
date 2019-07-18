package file

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/coredns/coredns/plugin/file/tree"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

/*
Create a zone with:

      apex
    /
   a MX
   a A

Test that: we create the proper tree and that delete
deletes the correct elements
*/

var tz = NewZone("example.org.", "db.example.org.")

type treebuf struct {
	*bytes.Buffer
}

func (t *treebuf) printFunc(e *tree.Elem, rrs map[uint16][]dns.RR) error {
	fmt.Fprintf(t.Buffer, "%v\n", rrs) // should be fixed order in new go versions.
	return nil
}

func TestZoneInsertAndDelete(t *testing.T) {
	tz.Insert(test.SOA("example.org. IN SOA 1 2 3 4 5"))

	if x := tz.Apex.SOA.Header().Name; x != "example.org." {
		t.Errorf("Failed to insert SOA, expected %s, git %s", "example.org.", x)
	}

	// Insert two RRs and then remove one.
	tz.Insert(test.A("a.example.org. IN A 127.0.0.1"))
	tz.Insert(test.MX("a.example.org. IN MX 10 mx.example.org."))

	tz.Delete(test.MX("a.example.org. IN MX 10 mx.example.org."))

	tb := treebuf{new(bytes.Buffer)}

	tz.Walk(tb.printFunc)
	if tb.String() != "map[1:[a.example.org.\t3600\tIN\tA\t127.0.0.1]]\n" {
		t.Errorf("Expected 1 A record in tree, got %s", tb.String())
	}

	tz.Delete(test.A("a.example.org. IN A 127.0.0.1"))

	tb.Reset()

	tz.Walk(tb.printFunc)
	if tb.String() != "" {
		t.Errorf("Expected no record in tree, got %s", tb.String())
	}
}
