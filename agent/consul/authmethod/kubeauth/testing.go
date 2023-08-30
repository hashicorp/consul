package kubeauth

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestAPIServer is a way to mock the Kubernetes API server as it is used by
// the consul kubernetes auth method.
//
//   - POST /apis/authentication.k8s.io/v1/tokenreviews
//   - GET  /api/v1/namespaces/<NAMESPACE>/serviceaccounts/<NAME>
type TestAPIServer struct {
	srv    *httptest.Server
	caCert string

	mu                       sync.Mutex
	authorizedJWT            string                 // token review and sa read
	allowedServiceAccountJWT string                 // general service account
	replyStatus              *authv1.TokenReview    // general service account
	replyRead                *corev1.ServiceAccount // general service account
}

// StartTestAPIServer creates a disposable TestAPIServer and binds it to a
// random free port.
func StartTestAPIServer(t testing.T) *TestAPIServer {
	s := &TestAPIServer{}
	s.srv = httptest.NewUnstartedServer(s)
	s.srv.Config.ErrorLog = log.New(ioutil.Discard, "", 0)
	s.srv.StartTLS()

	bs := s.srv.TLS.Certificates[0].Certificate[0]

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs}))
	s.caCert = buf.String()

	return s
}

// AuthorizeJWT allowlists the given JWT as able to use the API server.
func (s *TestAPIServer) AuthorizeJWT(jwt string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.authorizedJWT = jwt
}

// SetAllowedServiceAccount configures the singular known Service Account
// installed in this API server. If any of namespace/name/uid/jwt are empty
// it removes anything previously configured.
//
// It is up to the caller to ensure that the provided JWT matches the other
// data.
func (s *TestAPIServer) SetAllowedServiceAccount(
	namespace, name, uid, overrideAnnotation, jwt string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if namespace == "" || name == "" || uid == "" || jwt == "" {
		s.allowedServiceAccountJWT = ""
		s.replyStatus = nil
		s.replyRead = nil
		return
	}

	s.allowedServiceAccountJWT = jwt
	s.replyRead = createReadServiceAccountFound(namespace, name, uid, overrideAnnotation)
	s.replyStatus = createTokenReviewFound(namespace, name, uid, jwt)
}

// Stop stops the running TestAPIServer.
func (s *TestAPIServer) Stop() {
	s.srv.Close()
}

// Addr returns the current base URL for the running webserver.
func (s *TestAPIServer) Addr() string { return s.srv.URL }

// CACert returns the pem-encoded CA certificate used by the HTTPS server.
func (s *TestAPIServer) CACert() string { return s.caCert }

var readServiceAccountPathRE = regexp.MustCompile("^/api/v1/namespaces/([^/]+)/serviceaccounts/([^/]+)$")

func (s *TestAPIServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w.Header().Set("content-type", "application/json")

	if req.URL.Path == "/apis/authentication.k8s.io/v1/tokenreviews" {
		s.handleTokenReview(w, req)
		return
	}

	if m := readServiceAccountPathRE.FindStringSubmatch(req.URL.Path); m != nil {
		namespace, err := url.QueryUnescape(m[1])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		name, err := url.QueryUnescape(m[2])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.handleReadServiceAccount(namespace, name, w, req)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func writeJSON(w http.ResponseWriter, out interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}

func (s *TestAPIServer) handleTokenReview(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if auth, anon := s.isAuthenticated(req); !auth {
		var out interface{}
		if anon {
			out = createTokenReviewForbidden_NoAuthz()
		} else {
			out = createTokenReviewForbidden("default", "fake-account")
		}

		w.WriteHeader(http.StatusForbidden)
		if err := writeJSON(w, out); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if req.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var trReq authv1.TokenReview
	if err := json.Unmarshal(b, &trReq); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	reviewingJWT := trReq.Spec.Token

	var out interface{}
	if s.replyStatus == nil || reviewingJWT != s.allowedServiceAccountJWT {
		out = createTokenReviewNotFound(reviewingJWT)
	} else {
		out = s.replyStatus
	}
	w.WriteHeader(http.StatusCreated)

	if err := writeJSON(w, out); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *TestAPIServer) handleReadServiceAccount(
	namespace, name string,
	w http.ResponseWriter,
	req *http.Request,
) {
	if req.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var out interface{}
	if auth, anon := s.isAuthenticated(req); !auth {
		if anon {
			out = createReadServiceAccountForbidden_NoAuthz()
		} else {
			out = createReadServiceAccountForbidden(namespace, name)
		}
		w.WriteHeader(http.StatusForbidden)
	} else if s.replyRead == nil {
		out = createReadServiceAccountNotFound(name)
		w.WriteHeader(http.StatusNotFound)
	} else if s.replyRead.Namespace != namespace || s.replyRead.Name != name {
		out = createReadServiceAccountNotFound(name)
		w.WriteHeader(http.StatusNotFound)
	} else {
		out = s.replyRead
		w.WriteHeader(http.StatusOK)
	}

	if err := writeJSON(w, out); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *TestAPIServer) isAuthenticated(req *http.Request) (auth, anonymous bool) {
	authz := req.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return false, true
	}
	jwt := strings.TrimPrefix(authz, "Bearer ")

	return s.authorizedJWT == jwt, false
}

func createTokenReviewForbidden_NoAuthz() *metav1.Status {
	/*
	   STATUS: 403
	   {
	     "kind": "Status",
	     "apiVersion": "v1",
	     "metadata": {},
	     "status": "Failure",
	     "message": "tokenreviews.authentication.k8s.io is forbidden: User \"system:anonymous\" cannot create resource \"tokenreviews\" in API group \"authentication.k8s.io\" at the cluster scope",
	     "reason": "Forbidden",
	     "details": {
	       "group": "authentication.k8s.io",
	       "kind": "tokenreviews"
	     },
	     "code": 403
	   }
	*/
	return createStatus(
		metav1.StatusFailure,
		"tokenreviews.authentication.k8s.io is forbidden: User \"system:anonymous\" cannot create resource \"tokenreviews\" in API group \"authentication.k8s.io\" in the cluster scope",
		metav1.StatusReasonForbidden,
		&metav1.StatusDetails{
			Group: "authentication.k8s.io",
			Kind:  "tokenreviews",
		},
		403,
	)
}

func createTokenReviewForbidden(namespace, name string) *metav1.Status {
	/*
	   STATUS: 403
	   {
	     "kind": "Status",
	     "apiVersion": "v1",
	     "metadata": {},
	     "status": "Failure",
	     "message": "tokenreviews.authentication.k8s.io is forbidden: User \"system:serviceaccount:default:admin\" cannot create resource \"tokenreviews\" in API group \"authentication.k8s.io\" at the cluster scope",
	     "reason": "Forbidden",
	     "details": {
	       "group": "authentication.k8s.io",
	       "kind": "tokenreviews"
	     },
	     "code": 403
	   }
	*/
	return createStatus(
		metav1.StatusFailure,
		"tokenreviews.authentication.k8s.io is forbidden: User \"system:serviceaccount:"+namespace+":"+name+"\" cannot create resource \"tokenreviews\" in API group \"authentication.k8s.io\" in the cluster scope",
		metav1.StatusReasonForbidden,
		&metav1.StatusDetails{
			Group: "authentication.k8s.io",
			Kind:  "tokenreviews",
		},
		403,
	)
}

func createTokenReviewNotFound(jwt string) *authv1.TokenReview {
	/*
	   STATUS: 201
	   {
	     "kind": "TokenReview",
	     "apiVersion": "authentication.k8s.io/v1",
	     "metadata": {
	       "creationTimestamp": null
	     },
	     "spec": {
	       "token": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImZha2UtdG9rZW4tano2YnYiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZmFrZSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjgxYTY1Mjg2LTU3YzEtMTFlOS1iYzJhLTQ4ZTZjOGI4ZWNiNSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmZha2UifQ.DqjUXe34SzCP4NCwbhqV9EuksfzmTSLhJzkE_URyufeGJDn-Gw0_JS-_KmxZSdAO0XXNzB1tJNM1NCVW-V6YbThnPUw5WY4V2J6U1W72c2dzNBx_ipBxGBZ632ZnpViIRu6tL2guT36lWa8YnMDF_OY8sHhl_3kJ6MRxNxY41vAuf45mohi3gri46Kpzc3pf1g6PJ-0oogvUsZ2nBFv1mIdciGBV0zejMKc5Bnxur1L-hEQ9EgZrJ7o0yQRCWYgam_yo_M38EsB8b-suTzQJMA-pRgApOb9dHIV6YAE_b3g_pGkJjrPYzV4IJC1CiPfdz1SAjm7e0ARXtZmaoPltjQ"
	     },
	     "status": {
	       "user": {},
	       "error": "[invalid bearer token, Token has been invalidated]"
	     }
	   }
	*/
	return &authv1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TokenReview",
			APIVersion: "authentication.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: authv1.TokenReviewSpec{
			Token: jwt,
		},
		Status: authv1.TokenReviewStatus{
			User:  authv1.UserInfo{},
			Error: "[invalid bearer token, Token has been invalidated]",
		},
	}
}

func createTokenReviewFound(namespace, name, uid, jwt string) *authv1.TokenReview {
	/*
	   STATUS: 201
	   {
	     "kind": "TokenReview",
	     "apiVersion": "authentication.k8s.io/v1",
	     "metadata": {
	       "creationTimestamp": null
	     },
	     "spec": {
	       "token": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImRlbW8tdG9rZW4tbTljdm4iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZGVtbyIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjlmZjUxZmY0LTU1N2UtMTFlOS05Njg3LTQ4ZTZjOGI4ZWNiNSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmRlbW8ifQ.UJEphtrN261gy9WCl4ZKjm2PRDLDkc3Xg9VcDGfzyroOqFQ6sog5dVAb9voc5Nc0-H5b1yGwxDViEMucwKvZpA5pi7VEx_OskK-KTWXSmafM0Xg_AvzpU9Ed5TSRno-OhXaAraxdjXoC4myh1ay2DMeHUusJg_ibqcYJrWx-6MO1bH_ObORtAKhoST_8fzkqNAlZmsQ87FinQvYN5mzDXYukl-eeRdBgQUBkWvEb-Ju6cc0-QE4sUQ4IH_fs0fUyX_xc0om0SZGWLP909FTz4V8LxV8kr6L7irxROiS1jn3Fvyc9ur1PamVf3JOPPrOyfmKbaGRiWJM32b3buQw7cg"
	     },
	     "status": {
	       "authenticated": true,
	       "user": {
	         "username": "system:serviceaccount:default:demo",
	         "uid": "9ff51ff4-557e-11e9-9687-48e6c8b8ecb5",
	         "groups": [
	           "system:serviceaccounts",
	           "system:serviceaccounts:default",
	           "system:authenticated"
	         ]
	       }
	     }
	   }
	*/
	return &authv1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TokenReview",
			APIVersion: "authentication.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: authv1.TokenReviewSpec{
			Token: jwt,
		},
		Status: authv1.TokenReviewStatus{
			Authenticated: true,
			User: authv1.UserInfo{
				Username: "system:serviceaccount:" + namespace + ":" + name,
				UID:      uid,
				Groups: []string{
					"system:serviceaccounts",
					"system:serviceaccounts:default",
					"system:authenticated",
				},
			},
		},
	}
}

func createReadServiceAccountForbidden(namespace, name string) *metav1.Status {
	/*
		STATUS: 403
		{
		  "kind": "Status",
		  "apiVersion": "v1",
		  "metadata": {},
		  "status": "Failure",
		  "message": "serviceaccounts \"demo\" is forbidden: User \"system:serviceaccount:default:admin\" cannot get resource \"serviceaccounts\" in API group \"\" in the namespace \"default\"",
		  "reason": "Forbidden",
		  "details": {
		    "name": "demo",
		    "kind": "serviceaccounts"
		  },
		  "code": 403
		}
	*/
	return createStatus(
		metav1.StatusFailure,
		"serviceaccounts \""+name+"\" is forbidden: User \"system:serviceaccount:"+namespace+":"+name+"\" cannot get resource \"serviceaccounts\" in API group \"\" in the namespace \""+namespace+"\"",
		metav1.StatusReasonForbidden,
		&metav1.StatusDetails{
			Kind: "serviceaccounts",
			Name: name,
		},
		403,
	)
}

func createReadServiceAccountForbidden_NoAuthz() *metav1.Status {
	// missing bearer token header 403
	/*
	   {
	     "kind": "Status",
	     "apiVersion": "v1",
	     "metadata": {},
	     "status": "Failure",
	     "message": "serviceaccounts \"demo\" is forbidden: User \"system:anonymous\" cannot get resource \"serviceaccounts\" in API group \"\" in the namespace \"default\"",
	     "reason": "Forbidden",
	     "details": {
	       "name": "demo",
	       "kind": "serviceaccounts"
	     },
	     "code": 403
	   }
	*/
	return createStatus(
		metav1.StatusFailure,
		"serviceaccounts \"PLACEHOLDER\" is forbidden: User \"system:anonymous\" cannot get resource \"serviceaccounts\" in API group \"\" in the namespace \"default\"",
		metav1.StatusReasonForbidden,
		&metav1.StatusDetails{
			Kind: "serviceaccounts",
			Name: "PLACEHOLDER",
		},
		403,
	)
}

func createReadServiceAccountNotFound(name string) *metav1.Status {
	/*
	   STATUS: 404
	   {
	     "kind": "Status",
	     "apiVersion": "v1",
	     "metadata": {},
	     "status": "Failure",
	     "message": "serviceaccounts \"demo\" not found",
	     "reason": "NotFound",
	     "details": {
	       "name": "demo",
	       "kind": "serviceaccounts"
	     },
	     "code": 404
	   }
	*/
	return createStatus(
		metav1.StatusFailure,
		"serviceaccounts \""+name+"\" not found",
		metav1.StatusReasonNotFound,
		&metav1.StatusDetails{
			Kind: "serviceaccounts",
			Name: name,
		},
		404,
	)
}

func createReadServiceAccountFound(namespace, name, uid, overrideAnnotation string) *corev1.ServiceAccount {
	/*
	   STATUS: 200
	   {
	     "kind": "ServiceAccount",
	     "apiVersion": "v1",
	     "metadata": {
	       "name": "demo",
	       "namespace": "default",
	       "selfLink": "/api/v1/namespaces/default/serviceaccounts/demo",
	       "uid": "9ff51ff4-557e-11e9-9687-48e6c8b8ecb5",
	       "resourceVersion": "2101",
	       "creationTimestamp": "2019-04-02T19:36:34Z",
	       "annotations": {
	         "consul.hashicorp.com/service-name": "actual",
	         "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"ServiceAccount\",\"metadata\":{\"annotations\":{\"consul.hashicorp.com/service-name\":\"actual\"},\"name\":\"demo\",\"namespace\":\"default\"}}\n"
	       }
	     },
	     "secrets": [
	       {
	         "name": "demo-token-m9cvn"
	       }
	     ]
	   }
	*/
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			SelfLink:          "/api/v1/namespaces/" + namespace + "/serviceaccounts/" + name,
			UID:               types.UID(uid),
			ResourceVersion:   "123",
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
		Secrets: []corev1.ObjectReference{
			{
				Name: name + "-token-m9cvn",
			},
		},
	}
	if overrideAnnotation != "" {
		sa.ObjectMeta.Annotations = map[string]string{
			"consul.hashicorp.com/service-name": overrideAnnotation,
		}
	}

	return sa
}

func createStatus(status, message string, reason metav1.StatusReason, details *metav1.StatusDetails, code int32) *metav1.Status {
	return &metav1.Status{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Status",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{},
		Status:   status,
		Message:  message,
		Reason:   reason,
		Details:  details,
		Code:     code,
	}
}
