package sign

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/file/tree"

	"github.com/miekg/dns"
)

// write writes out the zone file to a temporary file which is then moved into the correct place.
func (s *Signer) write(z *file.Zone) error {
	f, err := ioutil.TempFile(s.directory, "signed-")
	if err != nil {
		return err
	}

	if err := write(f, z); err != nil {
		f.Close()
		return err
	}

	f.Close()
	return os.Rename(f.Name(), filepath.Join(s.directory, s.signedfile))
}

func write(w io.Writer, z *file.Zone) error {
	if _, err := io.WriteString(w, z.Apex.SOA.String()); err != nil {
		return err
	}
	w.Write([]byte("\n")) // RR Stringer() method doesn't include newline, which ends the RR in a zone file, write that here.
	for _, rr := range z.Apex.SIGSOA {
		io.WriteString(w, rr.String())
		w.Write([]byte("\n"))
	}
	for _, rr := range z.Apex.NS {
		io.WriteString(w, rr.String())
		w.Write([]byte("\n"))
	}
	for _, rr := range z.Apex.SIGNS {
		io.WriteString(w, rr.String())
		w.Write([]byte("\n"))
	}
	err := z.Walk(func(e *tree.Elem, _ map[uint16][]dns.RR) error {
		for _, r := range e.All() {
			io.WriteString(w, r.String())
			w.Write([]byte("\n"))
		}
		return nil
	})
	return err
}

// Parse parses the zone in filename and returns a new Zone or an error. This
// is similar to the Parse function in the *file* plugin. However when parsing
// the record types DNSKEY, RRSIG, CDNSKEY and CDS are *not* included in the returned
// zone (if encountered).
func Parse(f io.Reader, origin, fileName string) (*file.Zone, error) {
	zp := dns.NewZoneParser(f, dns.Fqdn(origin), fileName)
	zp.SetIncludeAllowed(true)
	z := file.NewZone(origin, fileName)
	seenSOA := false

	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		if err := zp.Err(); err != nil {
			return nil, err
		}

		switch rr.(type) {
		case *dns.DNSKEY, *dns.RRSIG, *dns.CDNSKEY, *dns.CDS:
			continue
		case *dns.SOA:
			seenSOA = true
			if err := z.Insert(rr); err != nil {
				return nil, err
			}
		default:
			if err := z.Insert(rr); err != nil {
				return nil, err
			}
		}
	}
	if !seenSOA {
		return nil, fmt.Errorf("file %q has no SOA record", fileName)
	}

	return z, nil
}
