package flags

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

type stringValue string

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}
func (s *stringValue) Get() interface{} { return string(*s) }
func (s *stringValue) String() string   { return string(*s) }

func TestFlagsPrintUsage(t *testing.T) {
	v := stringValue("default")
	f := flag.Flag{Name: "name", Usage: "usage", Value: &v, DefValue: "defvalue"}
	var w bytes.Buffer
	printFlag(&w, &f)
	expected := "  -name=<value>\n     usage (default)\n\n"
	require.Equal(t, expected, w.String())
}
