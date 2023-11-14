// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// package oidcauthtest exposes tools to assist in writing unit tests of OIDC
// and JWT authentication workflows.
//
// When the package is loaded it will randomly generate an ECDSA signing
// keypair used to sign JWTs both via the Server and the SignJWT method.
package oidcauthtest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/hashicorp/consul/internal/go-sso/oidcauth/internal/strutil"
)

// Server is local server the mocks the endpoints used by the OIDC and
// JWKS process.
type Server struct {
	httpServer *httptest.Server
	caCert     string
	returnFunc func()

	jwks                *jose.JSONWebKeySet
	allowedRedirectURIs []string
	replySubject        string
	replyUserinfo       map[string]interface{}

	mu                sync.Mutex
	clientID          string
	clientSecret      string
	expectedAuthCode  string
	expectedAuthNonce string
	customClaims      map[string]interface{}
	customAudience    string
	omitIDToken       bool
	disableUserInfo   bool
}

type TestingT interface {
	require.TestingT
	Helper()
	Cleanup(func())
}

// Start creates a disposable Server. If the port provided is
// zero it will bind to a random free port, otherwise the provided port is
// used.
func Start(t TestingT) *Server {
	t.Helper()
	s := &Server{
		allowedRedirectURIs: []string{
			"https://example.com",
		},
		replySubject: "r3qXcK2bix9eFECzsU3Sbmh0K16fatW6@clients",
		replyUserinfo: map[string]interface{}{
			"color":       "red",
			"temperature": "76",
			"flavor":      "umami",
		},
	}

	jwks, err := newJWKS(ecdsaPublicKey)
	require.NoError(t, err)
	s.jwks = jwks

	s.httpServer = httptest.NewUnstartedServer(s)
	s.httpServer.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.httpServer.StartTLS()
	t.Cleanup(s.httpServer.Close)

	cert := s.httpServer.Certificate()

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	s.caCert = buf.String()

	return s
}

// SetClientCreds is for configuring the client information required for the
// OIDC workflows.
func (s *Server) SetClientCreds(clientID, clientSecret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientID = clientID
	s.clientSecret = clientSecret
}

// SetExpectedAuthCode configures the auth code to return from /auth and the
// allowed auth code for /token.
func (s *Server) SetExpectedAuthCode(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expectedAuthCode = code
}

// SetExpectedAuthNonce configures the nonce value required for /auth.
func (s *Server) SetExpectedAuthNonce(nonce string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expectedAuthNonce = nonce
}

// SetAllowedRedirectURIs allows you to configure the allowed redirect URIs for
// the OIDC workflow. If not configured a sample of "https://example.com" is
// used.
func (s *Server) SetAllowedRedirectURIs(uris []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedRedirectURIs = uris
}

// SetCustomClaims lets you set claims to return in the JWT issued by the OIDC
// workflow.
func (s *Server) SetCustomClaims(customClaims map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customClaims = customClaims
}

// SetCustomAudience configures what audience value to embed in the JWT issued
// by the OIDC workflow.
func (s *Server) SetCustomAudience(customAudience string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customAudience = customAudience
}

// OmitIDTokens forces an error state where the /token endpoint does not return
// id_token.
func (s *Server) OmitIDTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.omitIDToken = true
}

// DisableUserInfo makes the userinfo endpoint return 404 and omits it from the
// discovery config.
func (s *Server) DisableUserInfo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disableUserInfo = true
}

// Stop stops the running Server.
func (s *Server) Stop() {
	s.httpServer.Close()
}

// Addr returns the current base URL for the running webserver.
func (s *Server) Addr() string { return s.httpServer.URL }

// CACert returns the pem-encoded CA certificate used by the HTTPS server.
func (s *Server) CACert() string { return s.caCert }

// SigningKeys returns the pem-encoded keys used to sign JWTs.
func (s *Server) SigningKeys() (pub, priv string) {
	return SigningKeys()
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	switch req.URL.Path {
	case "/.well-known/openid-configuration":
		if req.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		reply := struct {
			Issuer           string `json:"issuer"`
			AuthEndpoint     string `json:"authorization_endpoint"`
			TokenEndpoint    string `json:"token_endpoint"`
			JWKSURI          string `json:"jwks_uri"`
			UserinfoEndpoint string `json:"userinfo_endpoint,omitempty"`
		}{
			Issuer:           s.Addr(),
			AuthEndpoint:     s.Addr() + "/auth",
			TokenEndpoint:    s.Addr() + "/token",
			JWKSURI:          s.Addr() + "/certs",
			UserinfoEndpoint: s.Addr() + "/userinfo",
		}
		if s.disableUserInfo {
			reply.UserinfoEndpoint = ""
		}

		if err := writeJSON(w, &reply); err != nil {
			return
		}

	case "/auth":
		if req.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		qv := req.URL.Query()

		if qv.Get("response_type") != "code" {
			writeAuthErrorResponse(w, req, "unsupported_response_type", "")
			return
		}
		if qv.Get("scope") != "openid" {
			writeAuthErrorResponse(w, req, "invalid_scope", "")
			return
		}

		if s.expectedAuthCode == "" {
			writeAuthErrorResponse(w, req, "access_denied", "")
			return
		}

		nonce := qv.Get("nonce")
		if s.expectedAuthNonce != "" && s.expectedAuthNonce != nonce {
			writeAuthErrorResponse(w, req, "access_denied", "")
			return
		}

		state := qv.Get("state")
		if state == "" {
			writeAuthErrorResponse(w, req, "invalid_request", "missing state parameter")
			return
		}

		redirectURI := qv.Get("redirect_uri")
		if redirectURI == "" {
			writeAuthErrorResponse(w, req, "invalid_request", "missing redirect_uri parameter")
			return
		}

		redirectURI += "?state=" + url.QueryEscape(state) +
			"&code=" + url.QueryEscape(s.expectedAuthCode)

		http.Redirect(w, req, redirectURI, http.StatusFound)

		return

	case "/certs":
		if req.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := writeJSON(w, s.jwks); err != nil {
			return
		}

	case "/certs_missing":
		w.WriteHeader(http.StatusNotFound)

	case "/certs_invalid":
		w.Write([]byte("It's not a keyset!"))

	case "/token":
		if req.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		switch {
		case req.FormValue("grant_type") != "authorization_code":
			_ = writeTokenErrorResponse(w, req, http.StatusBadRequest, "invalid_request", "bad grant_type")
			return
		case !strutil.StrListContains(s.allowedRedirectURIs, req.FormValue("redirect_uri")):
			_ = writeTokenErrorResponse(w, req, http.StatusBadRequest, "invalid_request", "redirect_uri is not allowed")
			return
		case req.FormValue("code") != s.expectedAuthCode:
			_ = writeTokenErrorResponse(w, req, http.StatusUnauthorized, "invalid_grant", "unexpected auth code")
			return
		}

		stdClaims := jwt.Claims{
			Subject:   s.replySubject,
			Issuer:    s.Addr(),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-5 * time.Second)),
			Expiry:    jwt.NewNumericDate(time.Now().Add(5 * time.Second)),
			Audience:  jwt.Audience{s.clientID},
		}
		if s.customAudience != "" {
			stdClaims.Audience = jwt.Audience{s.customAudience}
		}

		jwtData, err := SignJWT("", stdClaims, s.customClaims)
		if err != nil {
			_ = writeTokenErrorResponse(w, req, http.StatusInternalServerError, "server_error", err.Error())
			return
		}

		reply := struct {
			AccessToken string `json:"access_token"`
			IDToken     string `json:"id_token,omitempty"`
		}{
			AccessToken: jwtData,
			IDToken:     jwtData,
		}
		if s.omitIDToken {
			reply.IDToken = ""
		}
		if err := writeJSON(w, &reply); err != nil {
			return
		}

	case "/userinfo":
		if s.disableUserInfo {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := writeJSON(w, s.replyUserinfo); err != nil {
			return
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeAuthErrorResponse(w http.ResponseWriter, req *http.Request, errorCode, errorMessage string) {
	qv := req.URL.Query()

	redirectURI := qv.Get("redirect_uri") +
		"?state=" + url.QueryEscape(qv.Get("state")) +
		"&error=" + url.QueryEscape(errorCode)

	if errorMessage != "" {
		redirectURI += "&error_description=" + url.QueryEscape(errorMessage)
	}

	http.Redirect(w, req, redirectURI, http.StatusFound)
}

func writeTokenErrorResponse(w http.ResponseWriter, req *http.Request, statusCode int, errorCode, errorMessage string) error {
	body := struct {
		Code string `json:"error"`
		Desc string `json:"error_description,omitempty"`
	}{
		Code: errorCode,
		Desc: errorMessage,
	}

	w.WriteHeader(statusCode)
	return writeJSON(w, &body)
}

// newJWKS converts a pem-encoded public key into JWKS data suitable for a
// verification endpoint response
func newJWKS(pubKey string) (*jose.JSONWebKeySet, error) {
	block, _ := pem.Decode([]byte(pubKey))
	if block == nil {
		return nil, fmt.Errorf("unable to decode public key")
	}
	input := block.Bytes

	pub, err := x509.ParsePKIXPublicKey(input)
	if err != nil {
		return nil, err
	}
	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key: pub,
			},
		},
	}, nil
}

func writeJSON(w http.ResponseWriter, out interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}

// SignJWT will bundle the provided claims into a signed JWT. The provided key
// is assumed to be ECDSA.
//
// If no private key is provided, the default package keys are used. These can
// be retrieved via the SigningKeys() method.
func SignJWT(privKey string, claims jwt.Claims, privateClaims interface{}) (string, error) {
	if privKey == "" {
		privKey = ecdsaPrivateKey
	}
	var key *ecdsa.PrivateKey
	block, _ := pem.Decode([]byte(privKey))
	if block != nil {
		var err error
		key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
	}

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", err
	}

	raw, err := jwt.Signed(sig).
		Claims(claims).
		Claims(privateClaims).
		CompactSerialize()
	if err != nil {
		return "", err
	}

	return raw, nil
}

// httptestNewUnstartedServerWithPort is roughly the same as
// httptest.NewUnstartedServer() but allows the caller to explicitly choose the
// port if desired.
func httptestNewUnstartedServerWithPort(handler http.Handler, port int) *httptest.Server {
	if port == 0 {
		return httptest.NewUnstartedServer(handler)
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Sprintf("httptest: failed to listen on a port: %v", err))
	}

	return &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: handler},
	}
}

// SigningKeys returns the pem-encoded keys used to sign JWTs by default.
func SigningKeys() (pub, priv string) {
	return ecdsaPublicKey, ecdsaPrivateKey
}

var (
	ecdsaPublicKey  string
	ecdsaPrivateKey string
)

func init() {
	// Each time we run tests we generate a unique set of keys for use in the
	// test.  These are cached between runs but do not persist between restarts
	// of the test binary.
	var err error
	ecdsaPublicKey, ecdsaPrivateKey, err = GenerateKey()
	if err != nil {
		panic(err)
	}
}

func GenerateKey() (pub, priv string, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("error generating private key: %v", err)
	}

	{
		derBytes, err := x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			return "", "", fmt.Errorf("error marshaling private key: %v", err)
		}
		pemBlock := &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: derBytes,
		}
		priv = string(pem.EncodeToMemory(pemBlock))
	}
	{
		derBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
		if err != nil {
			return "", "", fmt.Errorf("error marshaling public key: %v", err)
		}
		pemBlock := &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: derBytes,
		}
		pub = string(pem.EncodeToMemory(pemBlock))
	}

	return pub, priv, nil
}
