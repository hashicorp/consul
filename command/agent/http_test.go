package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"
)

func makeHTTPServer(t *testing.T) (string, *HTTPServer) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	server, err := NewHTTPServer(agent, agent.logOutput, conf.HTTPAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}

func encodeReq(obj interface{}) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(obj)
	return ioutil.NopCloser(buf)
}
