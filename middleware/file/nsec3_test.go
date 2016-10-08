package file

import (
	"strings"
	"testing"
)

func TestParseNSEC3PARAM(t *testing.T) {
	_, err := Parse(strings.NewReader(nsec3paramTest), "miek.nl", "stdin")
	if err == nil {
		t.Fatalf("expected error when reading zone, got nothing")
	}
}

func TestParseNSEC3(t *testing.T) {
	_, err := Parse(strings.NewReader(nsec3Test), "miek.nl", "stdin")
	if err == nil {
		t.Fatalf("expected error when reading zone, got nothing")
	}
}

const nsec3paramTest = `miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1460175181 14400 3600 604800 14400
miek.nl.		1800	IN	NS	omval.tednet.nl.
miek.nl.		0	IN	NSEC3PARAM 1 0 5 A3DEBC9CC4F695C7`

const nsec3Test = `example.org.		1800	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2016082508 7200 3600 1209600 3600
aub8v9ce95ie18spjubsr058h41n7pa5.example.org. 284 IN NSEC3 1 1 5 D0CBEAAF0AC77314 AUB95P93VPKP55G6U5S4SGS7LS61ND85 NS SOA TXT RRSIG DNSKEY NSEC3PARAM
aub8v9ce95ie18spjubsr058h41n7pa5.example.org. 284 IN RRSIG NSEC3 8 2 600 20160910232502 20160827231002 14028 example.org. XBNpA7KAIjorPbXvTinOHrc1f630aHic2U716GHLHA4QMx9cl9ss4QjR Wj2UpDM9zBW/jNYb1xb0yjQoez/Jv200w0taSWjRci5aUnRpOi9bmcrz STHb6wIUjUsbJ+NstQsUwVkj6679UviF1FqNwr4GlJnWG3ZrhYhE+NI6 s0k=`
