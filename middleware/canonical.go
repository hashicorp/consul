package middleware

import (
	"bytes"

	"github.com/miekg/dns"
)

// Less returns <0 when a is less than b, 0 when they are equal and
// >0 when a is larger than b.
// The function order names in DNSSEC canonical order.
//
// See http://bert-hubert.blogspot.co.uk/2015/10/how-to-do-fast-canonical-ordering-of.html
// for a blog article on how we do this. And https://tools.ietf.org/html/rfc4034#section-6.1 .
func Less(a, b string) int {
	i := 1
	aj := len(a)
	bj := len(b)
	for {
		ai, oka := dns.PrevLabel(a, i)
		bi, okb := dns.PrevLabel(b, i)
		if oka && okb {
			return 0
		}
		// sadly this []byte will allocate...
		ab := []byte(a[ai:aj])
		toLowerAndDDD(ab)
		bb := []byte(b[bi:bj])
		toLowerAndDDD(bb)

		res := bytes.Compare(ab, bb)
		if res != 0 {
			return res
		}

		i++
		aj, bj = ai, bi
	}
	return 0
}

func toLowerAndDDD(b []byte) {
	lb := len(b)
	for i := 0; i < lb; i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
			continue
		}
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
func dddToByte(s []byte) byte { return byte((s[1]-'0')*100 + (s[2]-'0')*10 + (s[3] - '0')) }
