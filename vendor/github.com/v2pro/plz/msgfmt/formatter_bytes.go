package msgfmt

import "unicode/utf8"

type bytesFormatter int

func (formatter bytesFormatter) Format(space []byte, properties []interface{}) []byte {
	return writeBytes(space, properties[formatter].([]byte))
}

// safeSet holds the value true if the ASCII character with the given array
// position can be represented inside a JSON string without any further
// escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), and the backslash character ("\").
var safeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      true,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      false,
	'}':      true,
	'~':      true,
	'\u007f': true,
}

var hex = "0123456789abcdef"

func writeBytes(space []byte, s []byte) []byte {
	// write string, the fast path, without utf8 and escape support
	var i int
	var c byte
	for i, c = range s {
		if c < utf8.RuneSelf && safeSet[c] {
			space = append(space, c)
		} else {
			break
		}
	}
	if i == len(s)-1 {
		return space
	}
	return writeBytesSlowPath(space, s[i:])
}

func writeBytesSlowPath(space []byte, s []byte) []byte {
	start := 0
	// for the remaining parts, we process them char by char
	var i int
	var b byte
	for i, b = range s {
		if b >= utf8.RuneSelf {
			space = append(space, '\\', 'x', hex[b>>4], hex[b&0xF])
			start = i + 1
			continue
		}
		if safeSet[b] {
			continue
		}
		if start < i {
			space = append(space, s[start:i]...)
		}
		switch b {
		case '\\', '"':
			space = append(space, '\\', b)
		case '\n':
			space = append(space, '\\', 'n')
		case '\r':
			space = append(space, '\\', 'r')
		case '\t':
			space = append(space, '\\', 't')
		default:
			// This encodes bytes < 0x20 except for \t, \n and \r.
			space = append(space, '\\', 'x', hex[b>>4], hex[b&0xF])
		}
		start = i + 1
	}
	if start < len(s) {
		space = append(space, s[start:]...)
	}
	return space
}
