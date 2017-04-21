// Package verify offers convenience routenes for content verification.
package verify

import (
	"bytes"
	"fmt"
	"path"
	"runtime"
	"strings"
)

// travel is the verification state
type travel struct {
	diffs []differ
}

// differ is a verification failure.
type differ struct {
	// path is the expression to the content.
	path string
	// msg has a reason.
	msg string
}

// segment is a differ.path component used for lazy formatting.
type segment struct {
	format string
	x      interface{}
}

func (t *travel) differ(path []*segment, msg string, args ...interface{}) {
	var buf bytes.Buffer
	for _, s := range path {
		buf.WriteString(fmt.Sprintf(s.format, s.x))
	}

	t.diffs = append(t.diffs, differ{
		msg:  fmt.Sprintf(msg, args...),
		path: buf.String(),
	})
}

func (t *travel) report(name string) string {
	if len(t.diffs) == 0 {
		return ""
	}

	var buf bytes.Buffer

	buf.WriteString("verification for ")
	buf.WriteString(name)
	if _, file, lineno, ok := runtime.Caller(2); ok {
		fmt.Fprintf(&buf, " at %s:%d", path.Base(file), lineno)
	}
	buf.WriteByte(':')

	for _, d := range t.diffs {
		buf.WriteByte('\n')
		if d.path != "" {
			buf.WriteString(d.path)
			buf.WriteString(": ")
		}
		lines := strings.Split(d.msg, "\n")
		buf.WriteString(lines[0])
		for _, l := range lines[1:] {
			buf.WriteByte('\n')
			buf.WriteString(strings.Repeat(" ", len(d.path)+2))
			buf.WriteString(l)
		}
	}

	return buf.String()
}
