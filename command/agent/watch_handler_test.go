package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	consultemplate "github.com/marouenj/consul-template/core"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/watch"
)

func TestVerifyWatchHandler(t *testing.T) {
	if err := verifyWatchHandler(nil); err == nil {
		t.Fatalf("should err")
	}
	if err := verifyWatchHandler(123); err == nil {
		t.Fatalf("should err")
	}
	if err := verifyWatchHandler([]string{"foo"}); err == nil {
		t.Fatalf("should err")
	}
	if err := verifyWatchHandler("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestMakeWatchHandler(t *testing.T) {
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

// scenario:
// - on consul start-up, up/down endpoint is undefined
// - put an undefined value
// - put "down"
// - put "up"
// - put "down"
// - put "up"
// - cause the template to render
// - cause the template to render
// - put "down"
// - cause the template to render
// - put "up"
func TestMakeWatchHandlerForArchetype_1(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Create a test template `dir`/redis.ctmpl
	ioutil.WriteFile(dir+"/redis.ctmpl", []byte("maxclients {{key \"archetype/redis-pool/redis1/maxclients\"}}"), 0777)

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	input := `{
		"archetype": {
			"id" : "redis1",
			"poolname" : "redis-pool",
			"tags" : ["redis pool"],
			"address" : "127.0.0.1",
			"port" : 7000,
			"check" : {
				"id" : "redis1",
				"name" : "redis@redis1",
				"script" : "redis-cli ping",
				"interval" : "10s"
			},
			"template" : {
				"source" : "` + dir + `/redis.ctmpl",
				"destination" : "` + dir + `/redis.conf",
				"startcommand" : "echo start > ` + dir + `/last-action.log",
				"restartcommand" : "echo restart > ` + dir + `/last-action.log",
				"stopcommand" : "echo stop > ` + dir + `/last-action.log"
			}
		}
	}`

	dec := json.NewDecoder(bytes.NewReader([]byte(input)))

	var raw interface{}
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := raw.(map[string]interface{}); !ok {
		t.Fatalf("err: %v", nil)
	}
	obj1 := raw.(map[string]interface{})

	if _, ok := obj1["archetype"]; !ok {
		t.Fatalf("err: %v", nil)
	}
	arch := obj1["archetype"]

	archetype, err := DecodeArchetypeDefinition(arch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	srv.agent.config.Archetypes = append(srv.agent.config.Archetypes, archetype)

	// Compile watch parameters
	// archetype/watch/{archetype_name}/{archetype_id}
	key := strings.Join([]string{"archetype", "watch", archetype.PoolName, archetype.ID}, "/")
	params := compileWatchParametersForArchetype(*srv.agent.config, key)
	wp, err := watch.Parse(params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	srv.agent.config.WatchPlansForArchetypes = append(srv.agent.config.WatchPlansForArchetypes, wp)

	// Allocate a slice of Runner
	srv.agent.config.Runners = make([]*consultemplate.Runner, len(srv.agent.config.Archetypes), len(srv.agent.config.Archetypes))

	// Register the watches for archetypes
	for i, wp := range srv.agent.config.WatchPlansForArchetypes {
		go func(wp *watch.WatchPlan) {
			var logOutput io.Writer
			logOutput = os.Stdout
			wp.Handler = makeWatchHandlerForArchetype(logOutput, srv.agent, i)
			wp.LogOutput = logOutput
			if err := wp.Run("127.0.0.1:18801"); err != nil {
				t.Fatalf("err: %v", err)
			}
		}(wp)
	}

	// PUT an undefined value
	{
		buf := bytes.NewBuffer([]byte("undefined"))
		req, err := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		if _, err := ioutil.ReadFile(dir + "/status"); err == nil {
			t.Fatalf("should not exist")
		}

		if _, err := ioutil.ReadFile(dir + "/redis.conf"); err == nil {
			t.Fatalf("should not exist")
		}
	}

	// PUT "down"
	{
		buf := bytes.NewBuffer([]byte("down"))
		req, err := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		if _, err := ioutil.ReadFile(dir + "/status"); err == nil {
			t.Fatalf("should not exist")
		}

		if _, err := ioutil.ReadFile(dir + "/redis.conf"); err == nil {
			t.Fatalf("should not exist")
		}
	}

	// PUT "up"
	{
		// put a val to template key
		buf := bytes.NewBuffer([]byte("1"))
		req, err := http.NewRequest("PUT", "/v1/kv/archetype/redis-pool/redis1/maxclients", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		// up
		buf = bytes.NewBuffer([]byte("up"))
		req, err = http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp = httptest.NewRecorder()
		obj2, err = srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read), "start") {
			t.Fatalf("bad: %v", string(read))
		}

		// check last rendered template
		read2, err := ioutil.ReadFile(dir + "/redis.conf")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read2), "maxclients 1") {
			t.Fatalf("bad: %v", string(read2))
		}
	}

	// PUT "down"
	{
		buf := bytes.NewBuffer([]byte("down"))
		req, err := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read), "stop") {
			t.Fatalf("bad: %v", string(read))
		}

		if _, err := ioutil.ReadFile(dir + "/redis.conf"); err == nil {
			t.Fatalf("should not exist")
		}
	}

	// PUT "up"
	{
		buf := bytes.NewBuffer([]byte("up"))
		req, err := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read), "start") {
			t.Fatalf("bad: %v", string(read))
		}

		// check last rendered template
		read2, err := ioutil.ReadFile(dir + "/redis.conf")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read2), "maxclients 1") {
			t.Fatalf("bad: %v", string(read2))
		}
	}

	// re-render
	{
		buf := bytes.NewBuffer([]byte("2"))
		req, err := http.NewRequest("PUT", "/v1/kv/archetype/redis-pool/redis1/maxclients", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read1, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read1), "restart") {
			t.Fatalf("bad: %v", string(read1))
		}

		// check last rendered template
		read2, err := ioutil.ReadFile(dir + "/redis.conf")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read2), "maxclients 2") {
			t.Fatalf("bad: %v", string(read2))
		}
	}

	// re-render
	{
		buf := bytes.NewBuffer([]byte("3"))
		req, err := http.NewRequest("PUT", "/v1/kv/archetype/redis-pool/redis1/maxclients", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read1, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read1), "restart") {
			t.Fatalf("bad: %v", string(read1))
		}

		// check last rendered template
		read2, err := ioutil.ReadFile(dir + "/redis.conf")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read2), "maxclients 3") {
			t.Fatalf("bad: %v", string(read2))
		}
	}

	// PUT "down"
	{
		buf := bytes.NewBuffer([]byte("down"))
		req, err := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read), "stop") {
			t.Fatalf("bad: %v", string(read))
		}

		if _, err := ioutil.ReadFile(dir + "/redis.conf"); err == nil {
			t.Fatalf("should not exist")
		}
	}

	// re-render
	{
		buf := bytes.NewBuffer([]byte("4"))
		req, err := http.NewRequest("PUT", "/v1/kv/archetype/redis-pool/redis1/maxclients", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action (should not be changed)
		read1, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read1), "stop") {
			t.Fatalf("bad: %v", string(read1))
		}

		// check last rendered template (should not exist)
		if _, err := ioutil.ReadFile(dir + "/redis.conf"); err == nil {
			t.Fatalf("should not exist")
		}
	}

	// PUT "up"
	{
		buf := bytes.NewBuffer([]byte("up"))
		req, err := http.NewRequest("PUT", "/v1/kv/"+key, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp := httptest.NewRecorder()
		obj2, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if res := obj2.(bool); !res {
			t.Fatalf("should work")
		}

		time.Sleep(time.Second)

		// check last action
		read, err := ioutil.ReadFile(dir + "/last-action.log")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read), "start") {
			t.Fatalf("bad: %v", string(read))
		}

		// check last rendered template
		read2, err := ioutil.ReadFile(dir + "/redis.conf")
		if err != nil {
			t.Fatalf("bad: %v", err)
		}
		if !strings.HasPrefix(string(read2), "maxclients 4") {
			t.Fatalf("bad: %v", string(read2))
		}
	}
}
