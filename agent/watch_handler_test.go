package agent

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestMakeWatchHandler(t *testing.T) {
	defer os.Remove("handler_out")
	defer os.Remove("handler_index_out")
	script := "bash -c 'echo $CONSUL_INDEX >> handler_index_out && cat >> handler_out'"
	handler := makeWatchHandler(testutil.Logger(t), script)
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
	handler := makeHTTPWatchHandler(testutil.Logger(t), &config)
	handler(100, []string{"foo", "bar", "baz"})
}

type raw map[string]interface{}

func TestMakeWatchPlan(t *testing.T) {
	type testCase struct {
		name        string
		params      map[string]interface{}
		expected    func(t *testing.T, plan *watch.Plan)
		expectedErr string
	}
	fn := func(t *testing.T, tc testCase) {
		plan, err := makeWatchPlan(hclog.New(nil), tc.params)
		if tc.expectedErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
			return
		}
		require.NoError(t, err)
		tc.expected(t, plan)
	}
	var testCases = []testCase{
		{
			name: "handler_type script, with deprecated handler field",
			params: raw{
				"type":         "key",
				"key":          "foo",
				"handler_type": "script",
				"handler":      "./script.sh",
			},
			expected: func(t *testing.T, plan *watch.Plan) {
				require.Equal(t, plan.HandlerType, "script")
				require.Equal(t, plan.Exempt["handler"], "./script.sh")
			},
		},
		{
			name: "handler_type script, with single arg",
			params: raw{
				"type":         "key",
				"key":          "foo",
				"handler_type": "script",
				"args":         "./script.sh",
			},
			expected: func(t *testing.T, plan *watch.Plan) {
				require.Equal(t, plan.HandlerType, "script")
				require.Equal(t, plan.Exempt["args"], []string{"./script.sh"})
			},
		},
		{
			name: "handler_type script, with multiple args from slice of interface",
			params: raw{
				"type":         "key",
				"key":          "foo",
				"handler_type": "script",
				"args":         []interface{}{"./script.sh", "arg1"},
			},
			expected: func(t *testing.T, plan *watch.Plan) {
				require.Equal(t, plan.HandlerType, "script")
				require.Equal(t, plan.Exempt["args"], []string{"./script.sh", "arg1"})
			},
		},
		{
			name: "handler_type script, with multiple args from slice of strings",
			params: raw{
				"type":         "key",
				"key":          "foo",
				"handler_type": "script",
				"args":         []string{"./script.sh", "arg1"},
			},
			expected: func(t *testing.T, plan *watch.Plan) {
				require.Equal(t, plan.HandlerType, "script")
				require.Equal(t, plan.Exempt["args"], []string{"./script.sh", "arg1"})
			},
		},
		{
			name: "handler_type script, with not string args",
			params: raw{
				"type":         "key",
				"key":          "foo",
				"handler_type": "script",
				"args":         []interface{}{"./script.sh", true},
			},
			expectedErr: "Watch args must be a list of strings",
		},
		{
			name: "conflicting handler",
			params: raw{
				"type":         "key",
				"key":          "foo",
				"handler_type": "script",
				"handler":      "./script.sh",
				"args":         []interface{}{"arg1"},
			},
			expectedErr: "Only one watch handler allowed",
		},
		{
			name: "no handler_type",
			params: raw{
				"type": "key",
				"key":  "foo",
			},
			expectedErr: "Must define a watch handler",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
