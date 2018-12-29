package flags

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"

	text "github.com/kr/text"
)

func Usage(txt string, flags *flag.FlagSet) string {
	u := &Usager{
		Usage: txt,
		Flags: flags,
	}
	return u.String()
}

type Usager struct {
	Usage string
	Flags *flag.FlagSet
}

func (u *Usager) String() string {
	out := new(bytes.Buffer)
	out.WriteString(strings.TrimSpace(u.Usage))
	out.WriteString("\n")
	out.WriteString("\n")

	if u.Flags != nil {
		f := &HTTPFlags{}
		clientFlags := f.ClientFlags()
		serverFlags := f.ServerFlags()

		var httpFlags, cmdFlags *flag.FlagSet
		u.Flags.VisitAll(func(f *flag.Flag) {
			if contains(clientFlags, f) || contains(serverFlags, f) {
				if httpFlags == nil {
					httpFlags = flag.NewFlagSet("", flag.ContinueOnError)
				}
				httpFlags.Var(f.Value, f.Name, f.Usage)
			} else {
				if cmdFlags == nil {
					cmdFlags = flag.NewFlagSet("", flag.ContinueOnError)
				}
				cmdFlags.Var(f.Value, f.Name, f.Usage)
			}
		})

		if httpFlags != nil {
			printTitle(out, "HTTP API Options")
			httpFlags.VisitAll(func(f *flag.Flag) {
				printFlag(out, f)
			})
		}

		if cmdFlags != nil {
			printTitle(out, "Command Options")
			cmdFlags.VisitAll(func(f *flag.Flag) {
				printFlag(out, f)
			})
		}
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

// contains returns true if the given flag is contained in the given flag
// set or false otherwise.
func contains(fs *flag.FlagSet, f *flag.Flag) bool {
	if fs == nil {
		return false
	}

	var in bool
	fs.VisitAll(func(hf *flag.Flag) {
		in = in || f.Name == hf.Name
	})
	return in
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
