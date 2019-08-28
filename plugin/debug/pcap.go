package debug

import (
	"bytes"
	"fmt"

	"github.com/coredns/coredns/plugin/pkg/log"

	"github.com/miekg/dns"
)

// Hexdump converts the dns message m to a hex dump Wireshark can import.
// See https://www.wireshark.org/docs/man-pages/text2pcap.html.
// This output looks like this:
//
// 00000 dc bd 01 00 00 01 00 00 00 00 00 01 07 65 78 61
// 000010 6d 70 6c 65 05 6c 6f 63 61 6c 00 00 01 00 01 00
// 000020 00 29 10 00 00 00 80 00 00 00
// 00002a
//
// Hexdump will use log.Debug to write the dump to the log, each line
// is prefixed with 'debug: ' so the data can be easily extracted.
//
// msg will prefix the pcap dump.
func Hexdump(m *dns.Msg, v ...interface{}) {
	if !log.D.Value() {
		return
	}

	buf, _ := m.Pack()
	if len(buf) == 0 {
		return
	}

	out := "\n" + string(hexdump(buf))
	v = append(v, out)
	log.Debug(v...)
}

// Hexdumpf dumps a DNS message as Hexdump, but allows a format string.
func Hexdumpf(m *dns.Msg, format string, v ...interface{}) {
	if !log.D.Value() {
		return
	}

	buf, _ := m.Pack()
	if len(buf) == 0 {
		return
	}

	format += "\n%s"
	v = append(v, hexdump(buf))
	log.Debugf(format, v...)
}

func hexdump(data []byte) []byte {
	b := new(bytes.Buffer)

	newline := ""
	for i := 0; i < len(data); i++ {
		if i%16 == 0 {
			fmt.Fprintf(b, "%s%s%06x", newline, prefix, i)
			newline = "\n"
		}
		fmt.Fprintf(b, " %02x", data[i])
	}
	fmt.Fprintf(b, "\n%s%06x", prefix, len(data))

	return b.Bytes()
}

const prefix = "debug: "
