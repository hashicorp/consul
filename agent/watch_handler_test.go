package agent

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/watch"
)

func TestMakeWatchHandler(t *testing.T) {
	t.Parallel()
	defer os.Remove("handler_out")
	defer os.Remove("handler_index_out")
	script := "bash -c 'echo $CONSUL_INDEX >> handler_index_out && cat >> handler_out'"
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

func TestMakeHTTPWatchHandler(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := r.Header.Get("X-Consul-Index")
		if idx != "100" {
			t.Fatalf("bad: %s", idx)
		}
		// Get the first one
		customHeader := r.Header.Get("X-Custom")
		if customHeader != "abc" {
			t.Fatalf("bad: %s", idx)
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if string(body) != "[\"foo\",\"bar\",\"baz\"]\n" {
			t.Fatalf("bad: %s", body)
		}
		w.Write([]byte("Ok, i see"))
	}))
	defer server.Close()
	config := watch.HttpHandlerConfig{
		Path:    server.URL,
		Header:  map[string][]string{"X-Custom": {"abc", "def"}},
		Timeout: time.Minute,
	}
	handler := makeHTTPWatchHandler(os.Stderr, &config)
	handler(100, []string{"foo", "bar", "baz"})
}
