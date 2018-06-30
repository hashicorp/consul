// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

const (
	TLSCAChainPath        = "testdata/tls-ca-chain.pem"
	ServerCertificatePath = "testdata/server.crt"
	ServerKeyPath         = "testdata/server.key"
	BarneyCertificatePath = "testdata/barney.crt"
	BarneyKeyNoPassPath   = "testdata/barney-no-pass.key"
	MissingCA             = "missing/ca.crt"
	MissingCert           = "missing/cert.crt"
	MissingKey            = "missing/secret.key"

	ExpectedMessage        = "I'm here to serve you!!!"
	BearerToken            = "theanswertothegreatquestionoflifetheuniverseandeverythingisfortytwo"
	BearerTokenFile        = "testdata/bearer.token"
	MissingBearerTokenFile = "missing/bearer.token"
	ExpectedBearer         = "Bearer " + BearerToken
	ExpectedUsername       = "arthurdent"
	ExpectedPassword       = "42"
)

var invalidHTTPClientConfigs = []struct {
	httpClientConfigFile string
	errMsg               string
}{
	{
		httpClientConfigFile: "testdata/http.conf.bearer-token-and-file-set.bad.yml",
		errMsg:               "at most one of bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.empty.bad.yml",
		errMsg:               "at most one of basic_auth, bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.basic-auth.too-much.bad.yaml",
		errMsg:               "at most one of basic_auth password & password_file must be configured",
	},
}

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, error) {
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(handler))

	tlsCAChain, err := ioutil.ReadFile(TLSCAChainPath)
	if err != nil {
		return nil, fmt.Errorf("Can't read %s", TLSCAChainPath)
	}
	serverCertificate, err := tls.LoadX509KeyPair(ServerCertificatePath, ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Can't load X509 key pair %s - %s", ServerCertificatePath, ServerKeyPath)
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(tlsCAChain)

	testServer.TLS = &tls.Config{
		Certificates: make([]tls.Certificate, 1),
		RootCAs:      rootCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootCAs}
	testServer.TLS.Certificates[0] = serverCertificate
	testServer.TLS.BuildNameToCertificate()

	testServer.StartTLS()

	return testServer, nil
}

func TestNewClientFromConfig(t *testing.T) {
	var newClientValidConfig = []struct {
		clientConfig HTTPClientConfig
		handler      func(w http.ResponseWriter, r *http.Request)
	}{
		{
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             "",
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: true},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		}, {
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		}, {
			clientConfig: HTTPClientConfig{
				BearerToken: BearerToken,
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		}, {
			clientConfig: HTTPClientConfig{
				BearerTokenFile: BearerTokenFile,
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		}, {
			clientConfig: HTTPClientConfig{
				BasicAuth: &BasicAuth{
					Username: ExpectedUsername,
					Password: ExpectedPassword,
				},
				TLSConfig: TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           BarneyCertificatePath,
					KeyFile:            BarneyKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				if !ok {
					fmt.Fprintf(w, "The Authorization header wasn't set")
				} else if ExpectedUsername != username {
					fmt.Fprintf(w, "The expected username (%s) differs from the obtained username (%s).", ExpectedUsername, username)
				} else if ExpectedPassword != password {
					fmt.Fprintf(w, "The expected password (%s) differs from the obtained password (%s).", ExpectedPassword, password)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
	}

	for _, validConfig := range newClientValidConfig {
		testServer, err := newTestServer(validConfig.handler)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer testServer.Close()

		client, err := NewClientFromConfig(validConfig.clientConfig, "test")
		if err != nil {
			t.Errorf("Can't create a client from this config: %+v", validConfig.clientConfig)
			continue
		}
		response, err := client.Get(testServer.URL)
		if err != nil {
			t.Errorf("Can't connect to the test server using this config: %+v", validConfig.clientConfig)
			continue
		}

		message, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Errorf("Can't read the server response body using this config: %+v", validConfig.clientConfig)
			continue
		}

		trimMessage := strings.TrimSpace(string(message))
		if ExpectedMessage != trimMessage {
			t.Errorf("The expected message (%s) differs from the obtained message (%s) using this config: %+v",
				ExpectedMessage, trimMessage, validConfig.clientConfig)
		}
	}
}

func TestNewClientFromInvalidConfig(t *testing.T) {
	var newClientInvalidConfig = []struct {
		clientConfig HTTPClientConfig
		errorMsg     string
	}{
		{
			clientConfig: HTTPClientConfig{
				TLSConfig: TLSConfig{
					CAFile:             MissingCA,
					CertFile:           "",
					KeyFile:            "",
					ServerName:         "",
					InsecureSkipVerify: true},
			},
			errorMsg: fmt.Sprintf("unable to use specified CA cert %s:", MissingCA),
		},
	}

	for _, invalidConfig := range newClientInvalidConfig {
		client, err := NewClientFromConfig(invalidConfig.clientConfig, "test")
		if client != nil {
			t.Errorf("A client instance was returned instead of nil using this config: %+v", invalidConfig.clientConfig)
		}
		if err == nil {
			t.Errorf("No error was returned using this config: %+v", invalidConfig.clientConfig)
		}
		if !strings.Contains(err.Error(), invalidConfig.errorMsg) {
			t.Errorf("Expected error %s does not contain %s", err.Error(), invalidConfig.errorMsg)
		}
	}
}

func TestMissingBearerAuthFile(t *testing.T) {
	cfg := HTTPClientConfig{
		BearerTokenFile: MissingBearerTokenFile,
		TLSConfig: TLSConfig{
			CAFile:             TLSCAChainPath,
			CertFile:           BarneyCertificatePath,
			KeyFile:            BarneyKeyNoPassPath,
			ServerName:         "",
			InsecureSkipVerify: false},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		} else {
			fmt.Fprint(w, ExpectedMessage)
		}
	}

	testServer, err := newTestServer(handler)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer testServer.Close()

	client, err := NewClientFromConfig(cfg, "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Get(testServer.URL)
	if err == nil {
		t.Fatal("No error is returned here")
	}

	if !strings.Contains(err.Error(), "unable to read bearer token file missing/bearer.token: open missing/bearer.token: no such file or directory") {
		t.Fatal("wrong error message being returned")
	}
}

func TestBearerAuthRoundTripper(t *testing.T) {
	const (
		newBearerToken = "goodbyeandthankyouforthefish"
	)

	fakeRoundTripper := NewRoundTripCheckRequest(func(req *http.Request) {
		bearer := req.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			t.Errorf("The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		}
	}, nil, nil)

	// Normal flow.
	bearerAuthRoundTripper := NewBearerAuthRoundTripper(BearerToken, fakeRoundTripper)
	request, _ := http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("User-Agent", "Douglas Adams mind")
	bearerAuthRoundTripper.RoundTrip(request)

	// Should honor already Authorization header set.
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization := NewBearerAuthRoundTripper(newBearerToken, fakeRoundTripper)
	request, _ = http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("Authorization", ExpectedBearer)
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization.RoundTrip(request)
}

func TestBearerAuthFileRoundTripper(t *testing.T) {
	const (
		newBearerToken = "goodbyeandthankyouforthefish"
	)

	fakeRoundTripper := NewRoundTripCheckRequest(func(req *http.Request) {
		bearer := req.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			t.Errorf("The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		}
	}, nil, nil)

	// Normal flow.
	bearerAuthRoundTripper := NewBearerAuthFileRoundTripper(BearerTokenFile, fakeRoundTripper)
	request, _ := http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("User-Agent", "Douglas Adams mind")
	bearerAuthRoundTripper.RoundTrip(request)

	// Should honor already Authorization header set.
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization := NewBearerAuthFileRoundTripper(MissingBearerTokenFile, fakeRoundTripper)
	request, _ = http.NewRequest("GET", "/hitchhiker", nil)
	request.Header.Set("Authorization", ExpectedBearer)
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization.RoundTrip(request)
}

func TestTLSConfig(t *testing.T) {
	configTLSConfig := TLSConfig{
		CAFile:             TLSCAChainPath,
		CertFile:           BarneyCertificatePath,
		KeyFile:            BarneyKeyNoPassPath,
		ServerName:         "localhost",
		InsecureSkipVerify: false}

	tlsCAChain, err := ioutil.ReadFile(TLSCAChainPath)
	if err != nil {
		t.Fatalf("Can't read the CA certificate chain (%s)",
			TLSCAChainPath)
	}
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(tlsCAChain)

	barneyCertificate, err := tls.LoadX509KeyPair(BarneyCertificatePath, BarneyKeyNoPassPath)
	if err != nil {
		t.Fatalf("Can't load the client key pair ('%s' and '%s'). Reason: %s",
			BarneyCertificatePath, BarneyKeyNoPassPath, err)
	}

	expectedTLSConfig := &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       []tls.Certificate{barneyCertificate},
		ServerName:         configTLSConfig.ServerName,
		InsecureSkipVerify: configTLSConfig.InsecureSkipVerify}
	expectedTLSConfig.BuildNameToCertificate()

	tlsConfig, err := NewTLSConfig(&configTLSConfig)
	if err != nil {
		t.Fatalf("Can't create a new TLS Config from a configuration (%s).", err)
	}

	if !reflect.DeepEqual(tlsConfig, expectedTLSConfig) {
		t.Fatalf("Unexpected TLS Config result: \n\n%+v\n expected\n\n%+v", tlsConfig, expectedTLSConfig)
	}
}

func TestTLSConfigEmpty(t *testing.T) {
	configTLSConfig := TLSConfig{
		CAFile:             "",
		CertFile:           "",
		KeyFile:            "",
		ServerName:         "",
		InsecureSkipVerify: true}

	expectedTLSConfig := &tls.Config{
		InsecureSkipVerify: configTLSConfig.InsecureSkipVerify}
	expectedTLSConfig.BuildNameToCertificate()

	tlsConfig, err := NewTLSConfig(&configTLSConfig)
	if err != nil {
		t.Fatalf("Can't create a new TLS Config from a configuration (%s).", err)
	}

	if !reflect.DeepEqual(tlsConfig, expectedTLSConfig) {
		t.Fatalf("Unexpected TLS Config result: \n\n%+v\n expected\n\n%+v", tlsConfig, expectedTLSConfig)
	}
}

func TestTLSConfigInvalidCA(t *testing.T) {
	var invalidTLSConfig = []struct {
		configTLSConfig TLSConfig
		errorMessage    string
	}{
		{
			configTLSConfig: TLSConfig{
				CAFile:             MissingCA,
				CertFile:           "",
				KeyFile:            "",
				ServerName:         "",
				InsecureSkipVerify: false},
			errorMessage: fmt.Sprintf("unable to use specified CA cert %s:", MissingCA),
		}, {
			configTLSConfig: TLSConfig{
				CAFile:             "",
				CertFile:           MissingCert,
				KeyFile:            BarneyKeyNoPassPath,
				ServerName:         "",
				InsecureSkipVerify: false},
			errorMessage: fmt.Sprintf("unable to use specified client cert (%s) & key (%s):", MissingCert, BarneyKeyNoPassPath),
		}, {
			configTLSConfig: TLSConfig{
				CAFile:             "",
				CertFile:           BarneyCertificatePath,
				KeyFile:            MissingKey,
				ServerName:         "",
				InsecureSkipVerify: false},
			errorMessage: fmt.Sprintf("unable to use specified client cert (%s) & key (%s):", BarneyCertificatePath, MissingKey),
		},
	}

	for _, anInvalididTLSConfig := range invalidTLSConfig {
		tlsConfig, err := NewTLSConfig(&anInvalididTLSConfig.configTLSConfig)
		if tlsConfig != nil && err == nil {
			t.Errorf("The TLS Config could be created even with this %+v", anInvalididTLSConfig.configTLSConfig)
			continue
		}
		if !strings.Contains(err.Error(), anInvalididTLSConfig.errorMessage) {
			t.Errorf("The expected error should contain %s, but got %s", anInvalididTLSConfig.errorMessage, err)
		}
	}
}

func TestBasicAuthNoPassword(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.basic-auth.no-password.yaml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	client, err := NewClientFromConfig(*cfg, "test")
	if err != nil {
		t.Errorf("Error creating HTTP Client: %v", err)
	}

	rt, ok := client.Transport.(*basicAuthRoundTripper)
	if !ok {
		t.Fatalf("Error casting to basic auth transport, %v", client.Transport)
	}

	if rt.username != "user" {
		t.Errorf("Bad HTTP client username: %s", rt.username)
	}
	if string(rt.password) != "" {
		t.Errorf("Expected empty HTTP client password: %s", rt.password)
	}
	if string(rt.passwordFile) != "" {
		t.Errorf("Expected empty HTTP client passwordFile: %s", rt.passwordFile)
	}
}

func TestBasicAuthPasswordFile(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.basic-auth.good.yaml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	client, err := NewClientFromConfig(*cfg, "test")
	if err != nil {
		t.Errorf("Error creating HTTP Client: %v", err)
	}

	rt, ok := client.Transport.(*basicAuthRoundTripper)
	if !ok {
		t.Errorf("Error casting to basic auth transport, %v", client.Transport)
	}

	if rt.username != "user" {
		t.Errorf("Bad HTTP client username: %s", rt.username)
	}
	if string(rt.password) != "" {
		t.Errorf("Bad HTTP client password: %s", rt.password)
	}
	if string(rt.passwordFile) != "testdata/basic-auth-password" {
		t.Errorf("Bad HTTP client passwordFile: %s", rt.passwordFile)
	}
}

func TestHideHTTPClientConfigSecrets(t *testing.T) {
	c, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error parsing %s: %s", "testdata/http.conf.good.yml", err)
	}

	// String method must not reveal authentication credentials.
	s := c.String()
	if strings.Contains(s, "mysecret") {
		t.Fatal("http client config's String method reveals authentication credentials.")
	}
}

func TestValidateHTTPConfig(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	err = cfg.Validate()
	if err != nil {
		t.Fatalf("Error validating %s: %s", "testdata/http.conf.good.yml", err)
	}
}

func TestInvalidHTTPConfigs(t *testing.T) {
	for _, ee := range invalidHTTPClientConfigs {
		_, _, err := LoadHTTPConfigFile(ee.httpClientConfigFile)
		if err == nil {
			t.Error("Expected error with config but got none")
			continue
		}
		if !strings.Contains(err.Error(), ee.errMsg) {
			t.Errorf("Expected error for invalid HTTP client configuration to contain %q but got: %s", ee.errMsg, err)
		}
	}
}

// LoadHTTPConfig parses the YAML input s into a HTTPClientConfig.
func LoadHTTPConfig(s string) (*HTTPClientConfig, error) {
	cfg := &HTTPClientConfig{}
	err := yaml.UnmarshalStrict([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadHTTPConfigFile parses the given YAML file into a HTTPClientConfig.
func LoadHTTPConfigFile(filename string) (*HTTPClientConfig, []byte, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := LoadHTTPConfig(string(content))
	if err != nil {
		return nil, nil, err
	}
	return cfg, content, nil
}

type roundTrip struct {
	theResponse *http.Response
	theError    error
}

func (rt *roundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.theResponse, rt.theError
}

type roundTripCheckRequest struct {
	checkRequest func(*http.Request)
	roundTrip
}

func (rt *roundTripCheckRequest) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.checkRequest(r)
	return rt.theResponse, rt.theError
}

// NewRoundTripCheckRequest creates a new instance of a type that implements http.RoundTripper,
// which before returning theResponse and theError, executes checkRequest against a http.Request.
func NewRoundTripCheckRequest(checkRequest func(*http.Request), theResponse *http.Response, theError error) http.RoundTripper {
	return &roundTripCheckRequest{
		checkRequest: checkRequest,
		roundTrip: roundTrip{
			theResponse: theResponse,
			theError:    theError}}
}
