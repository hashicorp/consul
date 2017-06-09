package agent

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMakeWatchHandler(t *testing.T) {
	t.Parallel()
	defer os.Remove("handler_out")
	defer os.Remove("handler_index_out")
	script := "echo $CONSUL_INDEX >> handler_index_out && cat >> handler_out"
	handler := makeWatchHandler(os.Stderr, script)
	handler(100, []string{"foo", "bar", "baz"})
	raw, err := ioutil.ReadFile("handler_out")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(raw) != "[\"foo\",\"bar\",\"baz\"]\n" {
		t.Fatalf("bad: %s", raw)
	}
	raw, err = ioutil.ReadFile("handler_index_out")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(raw) != "100\n" {
		t.Fatalf("bad: %s", raw)
	}
}
