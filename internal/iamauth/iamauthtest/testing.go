package iamauthtest

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// NewTestServer returns a fake AWS API server for local tests:
// It supports the following paths:
//   /sts returns STS API responses
//   /iam returns IAM API responses
func NewTestServer(s *Server) *httptest.Server {
	return httptest.NewUnstartedServer(s)
}

// Server contains configuration for the fake AWS API server.
type Server struct {
	// interface{} types to avoid import cycle
	GetCallerIdentityResponse interface{}
	GetRoleResponse           interface{}
	GetUserResponse           interface{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/sts"):
		writeXML(w, s.GetCallerIdentityResponse)
	case strings.HasPrefix(r.URL.Path, "/iam"):
		if bodyBytes, err := io.ReadAll(r.Body); err == nil {
			body := string(bodyBytes)
			switch {
			case strings.Contains(body, "Action=GetRole"):
				writeXML(w, s.GetRoleResponse)
				return
			case strings.Contains(body, "Action=GetUser"):
				writeXML(w, s.GetUserResponse)
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeXML(w http.ResponseWriter, val interface{}) {
	str, err := xml.MarshalIndent(val, "", " ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
	}
	w.Header().Add("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, string(str))
}
