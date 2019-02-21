package tree

import (
	"bytes"

	"github.com/miekg/dns"
)

// less returns <0 when a is less than b, 0 when they are equal and
// >0 when a is larger than b.
// The function orders names in DNSSEC canonical order: RFC 4034s section-6.1
//
// See https://bert-hubert.blogspot.co.uk/2015/10/how-to-do-fast-canonical-ordering-of.html
// for a blog article on this implementation, although here we still go label by label.
//
// The values of a and b are *not* lowercased before the comparison!
func less(a, b string) int {
	i := 1
	aj := len(a)
	bj := len(b)
	for {
		ai, oka := dns.PrevLabel(a, i)
		bi, okb := dns.PrevLabel(b, i)
		if oka && okb {
			return 0
		}

		// sadly this []byte will allocate... TODO(miek): check if this is needed
		// for a name, otherwise compare the strings.
		ab := []byte(a[ai:aj])
		bb := []byte(b[bi:bj])
		doDDD(ab)
		doDDD(bb)

		res := bytes.Compare(ab, bb)
		if res != 0 {
			return res
		}

		i++
		aj, bj = ai, bi
	}
}

func doDDD(b []byte) {
	lb := len(b)
	for i := 0; i < lb; i++ {
		if i+3 < lb && b[i] == '\\' && isDigit(b[i+1]) && isDigit(b[i+2]) && isDigit(b[i+3]) {
			b[i] = dddToByte(b[i:])
			for j := i + 1; j < lb-3; j++ {
				b[j] = b[j+3]
			}
			lb -= 3
		}
	}
}

func isDigit(b byte) bool     { return b >= '0' && b <= '9' }
func dddToByte(s []byte) byte { return (s[1]-'0')*100 + (s[2]-'0')*10 + (s[3] - '0') }
