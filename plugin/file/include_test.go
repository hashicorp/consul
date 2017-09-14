package file

import (
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/test"
)

// Make sure the external miekg/dns dependency is up to date

func TestInclude(t *testing.T) {

	name, rm, err := test.TempFile(".", "foo\tIN\tA\t127.0.0.1\n")
	if err != nil {
		t.Fatalf("Unable to create tmpfile %q: %s", name, err)
	}
	defer rm()

	zone := `$ORIGIN example.org.
@ IN SOA sns.dns.icann.org. noc.dns.icann.org. 2017042766 7200 3600 1209600 3600
$INCLUDE ` + name + "\n"

	z, err := Parse(strings.NewReader(zone), "example.org.", "test", 0)
	if err != nil {
		t.Errorf("Unable to parse zone %q: %s", "example.org.", err)
	}

	if _, ok := z.Search("foo.example.org."); !ok {
		t.Errorf("Failed to find %q in parsed zone", "foo.example.org.")
	}
}
