package flags

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"

	text "github.com/tonnerre/golang-text"
)

func Usage(txt string, cmdFlags, clientFlags, serverFlags *flag.FlagSet) string {
	u := &Usager{
		Usage:           txt,
		CmdFlags:        cmdFlags,
		HTTPClientFlags: clientFlags,
		HTTPServerFlags: serverFlags,
	}
	return u.String()
}

type Usager struct {
	Usage           string
	CmdFlags        *flag.FlagSet
	HTTPClientFlags *flag.FlagSet
	HTTPServerFlags *flag.FlagSet
}

func (u *Usager) String() string {
	out := new(bytes.Buffer)
	out.WriteString(strings.TrimSpace(u.Usage))
	out.WriteString("\n")
	out.WriteString("\n")

	httpFlags := u.HTTPClientFlags
	if httpFlags == nil {
		httpFlags = u.HTTPServerFlags
	} else {
		Merge(httpFlags, u.HTTPServerFlags)
	}

	if httpFlags != nil {
		printTitle(out, "HTTP API Options")
		httpFlags.VisitAll(func(f *flag.Flag) {
			printFlag(out, f)
		})
	}

	if u.CmdFlags != nil {
		printTitle(out, "Command Options")
		u.CmdFlags.VisitAll(func(f *flag.Flag) {
			if flagContains(httpFlags, f) {
				return
			}
			printFlag(out, f)
		})
	}

	return strings.TrimRight(out.String(), "\n")
}

// printTitle prints a consistently-formatted title to the given writer.
func printTitle(w io.Writer, s string) {
	fmt.Fprintf(w, "%s\n\n", s)
}

// printFlag prints a single flag to the given writer.
func printFlag(w io.Writer, f *flag.Flag) {
	example, _ := flag.UnquoteUsage(f)
	if example != "" {
		fmt.Fprintf(w, "  -%s=<%s>\n", f.Name, example)
	} else {
		fmt.Fprintf(w, "  -%s\n", f.Name)
	}

	indented := wrapAtLength(f.Usage, 5)
	fmt.Fprintf(w, "%s\n\n", indented)
}

// flagContains returns true if the given flag is contained in the given flag
// set or false otherwise.
func flagContains(fs *flag.FlagSet, f *flag.Flag) bool {
	if fs == nil {
		return false
	}

	var skip bool
	fs.VisitAll(func(hf *flag.Flag) {
		if skip {
			return
		}

		if f.Name == hf.Name {
			skip = true
			return
		}
	})

	return skip
}

// maxLineLength is the maximum width of any line.
const maxLineLength int = 72

// wrapAtLength wraps the given text at the maxLineLength, taking into account
// any provided left padding.
func wrapAtLength(s string, pad int) string {
	wrapped := text.Wrap(s, maxLineLength-pad)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = strings.Repeat(" ", pad) + line
	}
	return strings.Join(lines, "\n")
}
