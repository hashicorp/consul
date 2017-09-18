package agent

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
	if string(raw) != `["foo","bar","baz"]\n` {
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
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if string(body) != `["foo","bar","baz"]` {
			t.Fatalf("bad: %s", body)
		}
		w.Write([]byte("Ok, i see"))
		t.Log("goood") // TODO: Clean
	}))
	defer server.Close()
	t.Log("ww")
	handler := makeHTTPWatchHandler(os.Stderr, server.URL)
	handler(100, []string{"foo", "bar", "baz"})
	t.Log("uu")
}
