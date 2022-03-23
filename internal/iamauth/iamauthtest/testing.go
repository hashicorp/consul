package iamauthtest

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/hashicorp/consul/internal/iamauth/responses"
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
	GetCallerIdentityResponse responses.GetCallerIdentityResponse
	GetRoleResponse           responses.GetRoleResponse
	GetUserResponse           responses.GetUserResponse
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusBadRequest, r)
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
		writeError(w, http.StatusBadRequest, r)
	default:
		writeError(w, http.StatusNotFound, r)
	}
}

func writeXML(w http.ResponseWriter, val interface{}) {
	str, err := xml.MarshalIndent(val, "", " ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	w.Header().Add("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(str))
}

func writeError(w http.ResponseWriter, code int, r *http.Request) {
	w.WriteHeader(code)
	msg := fmt.Sprintf("%s %s", r.Method, r.URL)
	fmt.Fprintf(w, `<ErrorResponse xmlns="https://fakeaws/">
  <Error>
	<Message>Fake AWS Server Error: %s</Message>
  </Error>
</ErrorResponse>`, msg)
}
