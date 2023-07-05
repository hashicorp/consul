package agent

import (
	"net/http"
)

func (s *HTTPHandlers) CloudLink(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "PUT":
		return s.cloudLinkPut(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}
}

// configGet gets either a specific config entry, or lists all config entries
// of a kind if no name is provided.
func (s *HTTPHandlers) cloudLinkPut(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// parse values
	var clientID string
	if clientID = req.URL.Query().Get("client_id"); clientID == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing client ID"}
	}

	var secretID string
	if secretID = req.URL.Query().Get("secret_id"); secretID == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing client ID"}
	}

	var resourceID string
	if resourceID = req.URL.Query().Get("resource_id"); resourceID == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing client ID"}
	}

	// TODO: write as a config to disk

	// TODO: restart

	return true, nil
}
