package agent

import (
	"fmt"
	"net/http"
)

func (s *HTTPHandlers) ACLLegacy(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	resp.WriteHeader(http.StatusGone)
	msg := "Endpoint %v for the legacy ACL system was removed in Consul 1.11."
	fmt.Fprintf(resp, msg, req.URL.Path)
	return nil, nil
}
