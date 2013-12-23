package agent

import (
	"testing"
)

func makeHTTPServer(t *testing.T) (string, *HTTPServer) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	server, err := NewServer(agent, agent.logOutput, conf.HTTPAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir, server
}
