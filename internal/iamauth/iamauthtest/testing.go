package iamauthtest

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/consul/internal/iamauth/responses"
	"github.com/hashicorp/consul/internal/iamauth/responsestest"
)

// NewTestServer returns a fake AWS API server for local tests:
// It supports the following paths:
//   /sts returns STS API responses
//   /iam returns IAM API responses
func NewTestServer(t *testing.T, s *Server) *httptest.Server {
	server := httptest.NewUnstartedServer(s)
	t.Cleanup(server.Close)
	server.Start()
	return server
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

type Fixture struct {
	AssumedRoleARN   string
	CanonicalRoleARN string
	RoleARN          string
	RoleARNWildcard  string
	RoleName         string
	RolePath         string
	RoleTags         map[string]string

	EntityID            string
	EntityIDWithSession string
	AccountID           string

	UserARN         string
	UserARNWildcard string
	UserName        string
	UserPath        string
	UserTags        map[string]string

	ServerForRole *Server
	ServerForUser *Server
}

func MakeFixture() Fixture {
	f := Fixture{
		AssumedRoleARN:   "arn:aws:sts::1234567890:assumed-role/my-role/some-session",
		CanonicalRoleARN: "arn:aws:iam::1234567890:role/my-role",
		RoleARN:          "arn:aws:iam::1234567890:role/some/path/my-role",
		RoleARNWildcard:  "arn:aws:iam::1234567890:role/some/path/*",
		RoleName:         "my-role",
		RolePath:         "some/path",
		RoleTags: map[string]string{
			"service-name": "my-service",
			"env":          "my-env",
		},

		EntityID:            "AAAsomeuniqueid",
		EntityIDWithSession: "AAAsomeuniqueid:some-session",
		AccountID:           "1234567890",

		UserARN:         "arn:aws:iam::1234567890:user/my-user",
		UserARNWildcard: "arn:aws:iam::1234567890:user/*",
		UserName:        "my-user",
		UserPath:        "",
		UserTags:        map[string]string{"user-group": "my-group"},
	}

	f.ServerForRole = &Server{
		GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(
			f.AssumedRoleARN, f.EntityIDWithSession, f.AccountID,
		),
		GetRoleResponse: responsestest.MakeGetRoleResponse(
			f.RoleARN, f.EntityID, toTags(f.RoleTags),
		),
	}

	f.ServerForUser = &Server{
		GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(
			f.UserARN, f.EntityID, f.AccountID,
		),
		GetUserResponse: responsestest.MakeGetUserResponse(
			f.UserARN, f.EntityID, toTags(f.UserTags),
		),
	}

	return f
}

func (f *Fixture) RoleTagKeys() []string   { return keys(f.RoleTags) }
func (f *Fixture) UserTagKeys() []string   { return keys(f.UserTags) }
func (f *Fixture) RoleTagValues() []string { return values(f.RoleTags) }
func (f *Fixture) UserTagValues() []string { return values(f.UserTags) }

// toTags converts the map to a slice of responses.Tag
func toTags(tags map[string]string) responses.Tags {
	members := []responses.TagMember{}
	for k, v := range tags {
		members = append(members, responses.TagMember{
			Key:   k,
			Value: v,
		})
	}
	return responses.Tags{Members: members}

}

// keys returns the keys in sorted order
func keys(tags map[string]string) []string {
	result := []string{}
	for k := range tags {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// values returns values in tags, ordered by sorted keys
func values(tags map[string]string) []string {
	result := []string{}
	for _, k := range keys(tags) { // ensures sorted by key
		result = append(result, tags[k])
	}
	return result
}
