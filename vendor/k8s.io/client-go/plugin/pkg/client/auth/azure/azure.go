/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/util/net"
	restclient "k8s.io/client-go/rest"
)

const (
	azureTokenKey = "azureTokenKey"
	tokenType     = "Bearer"
	authHeader    = "Authorization"

	cfgClientID     = "client-id"
	cfgTenantID     = "tenant-id"
	cfgAccessToken  = "access-token"
	cfgRefreshToken = "refresh-token"
	cfgExpiresIn    = "expires-in"
	cfgExpiresOn    = "expires-on"
	cfgEnvironment  = "environment"
	cfgApiserverID  = "apiserver-id"
)

func init() {
	if err := restclient.RegisterAuthProviderPlugin("azure", newAzureAuthProvider); err != nil {
		klog.Fatalf("Failed to register azure auth plugin: %v", err)
	}
}

var cache = newAzureTokenCache()

type azureTokenCache struct {
	lock  sync.Mutex
	cache map[string]*azureToken
}

func newAzureTokenCache() *azureTokenCache {
	return &azureTokenCache{cache: make(map[string]*azureToken)}
}

func (c *azureTokenCache) getToken(tokenKey string) *azureToken {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.cache[tokenKey]
}

func (c *azureTokenCache) setToken(tokenKey string, token *azureToken) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[tokenKey] = token
}

func newAzureAuthProvider(_ string, cfg map[string]string, persister restclient.AuthProviderConfigPersister) (restclient.AuthProvider, error) {
	var ts tokenSource

	environment, err := azure.EnvironmentFromName(cfg[cfgEnvironment])
	if err != nil {
		environment = azure.PublicCloud
	}
	ts, err = newAzureTokenSourceDeviceCode(environment, cfg[cfgClientID], cfg[cfgTenantID], cfg[cfgApiserverID])
	if err != nil {
		return nil, fmt.Errorf("creating a new azure token source for device code authentication: %v", err)
	}
	cacheSource := newAzureTokenSource(ts, cache, cfg, persister)

	return &azureAuthProvider{
		tokenSource: cacheSource,
	}, nil
}

type azureAuthProvider struct {
	tokenSource tokenSource
}

func (p *azureAuthProvider) Login() error {
	return errors.New("not yet implemented")
}

func (p *azureAuthProvider) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &azureRoundTripper{
		tokenSource:  p.tokenSource,
		roundTripper: rt,
	}
}

type azureRoundTripper struct {
	tokenSource  tokenSource
	roundTripper http.RoundTripper
}

var _ net.RoundTripperWrapper = &azureRoundTripper{}

func (r *azureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.Header.Get(authHeader)) != 0 {
		return r.roundTripper.RoundTrip(req)
	}

	token, err := r.tokenSource.Token()
	if err != nil {
		klog.Errorf("Failed to acquire a token: %v", err)
		return nil, fmt.Errorf("acquiring a token for authorization header: %v", err)
	}

	// clone the request in order to avoid modifying the headers of the original request
	req2 := new(http.Request)
	*req2 = *req
	req2.Header = make(http.Header, len(req.Header))
	for k, s := range req.Header {
		req2.Header[k] = append([]string(nil), s...)
	}

	req2.Header.Set(authHeader, fmt.Sprintf("%s %s", tokenType, token.token.AccessToken))

	return r.roundTripper.RoundTrip(req2)
}

func (r *azureRoundTripper) WrappedRoundTripper() http.RoundTripper { return r.roundTripper }

type azureToken struct {
	token       adal.Token
	environment string
	clientID    string
	tenantID    string
	apiserverID string
}

type tokenSource interface {
	Token() (*azureToken, error)
}

type azureTokenSource struct {
	source    tokenSource
	cache     *azureTokenCache
	lock      sync.Mutex
	cfg       map[string]string
	persister restclient.AuthProviderConfigPersister
}

func newAzureTokenSource(source tokenSource, cache *azureTokenCache, cfg map[string]string, persister restclient.AuthProviderConfigPersister) tokenSource {
	return &azureTokenSource{
		source:    source,
		cache:     cache,
		cfg:       cfg,
		persister: persister,
	}
}

// Token fetches a token from the cache of configuration if present otherwise
// acquires a new token from the configured source. Automatically refreshes
// the token if expired.
func (ts *azureTokenSource) Token() (*azureToken, error) {
	ts.lock.Lock()
	defer ts.lock.Unlock()

	var err error
	token := ts.cache.getToken(azureTokenKey)
	if token == nil {
		token, err = ts.retrieveTokenFromCfg()
		if err != nil {
			token, err = ts.source.Token()
			if err != nil {
				return nil, fmt.Errorf("acquiring a new fresh token: %v", err)
			}
		}
		if !token.token.IsExpired() {
			ts.cache.setToken(azureTokenKey, token)
			err = ts.storeTokenInCfg(token)
			if err != nil {
				return nil, fmt.Errorf("storing the token in configuration: %v", err)
			}
		}
	}
	if token.token.IsExpired() {
		token, err = ts.refreshToken(token)
		if err != nil {
			return nil, fmt.Errorf("refreshing the expired token: %v", err)
		}
		ts.cache.setToken(azureTokenKey, token)
		err = ts.storeTokenInCfg(token)
		if err != nil {
			return nil, fmt.Errorf("storing the refreshed token in configuration: %v", err)
		}
	}
	return token, nil
}

func (ts *azureTokenSource) retrieveTokenFromCfg() (*azureToken, error) {
	accessToken := ts.cfg[cfgAccessToken]
	if accessToken == "" {
		return nil, fmt.Errorf("no access token in cfg: %s", cfgAccessToken)
	}
	refreshToken := ts.cfg[cfgRefreshToken]
	if refreshToken == "" {
		return nil, fmt.Errorf("no refresh token in cfg: %s", cfgRefreshToken)
	}
	environment := ts.cfg[cfgEnvironment]
	if environment == "" {
		return nil, fmt.Errorf("no environment in cfg: %s", cfgEnvironment)
	}
	clientID := ts.cfg[cfgClientID]
	if clientID == "" {
		return nil, fmt.Errorf("no client ID in cfg: %s", cfgClientID)
	}
	tenantID := ts.cfg[cfgTenantID]
	if tenantID == "" {
		return nil, fmt.Errorf("no tenant ID in cfg: %s", cfgTenantID)
	}
	apiserverID := ts.cfg[cfgApiserverID]
	if apiserverID == "" {
		return nil, fmt.Errorf("no apiserver ID in cfg: %s", apiserverID)
	}
	expiresIn := ts.cfg[cfgExpiresIn]
	if expiresIn == "" {
		return nil, fmt.Errorf("no expiresIn in cfg: %s", cfgExpiresIn)
	}
	expiresOn := ts.cfg[cfgExpiresOn]
	if expiresOn == "" {
		return nil, fmt.Errorf("no expiresOn in cfg: %s", cfgExpiresOn)
	}

	return &azureToken{
		token: adal.Token{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    json.Number(expiresIn),
			ExpiresOn:    json.Number(expiresOn),
			NotBefore:    json.Number(expiresOn),
			Resource:     fmt.Sprintf("spn:%s", apiserverID),
			Type:         tokenType,
		},
		environment: environment,
		clientID:    clientID,
		tenantID:    tenantID,
		apiserverID: apiserverID,
	}, nil
}

func (ts *azureTokenSource) storeTokenInCfg(token *azureToken) error {
	newCfg := make(map[string]string)
	newCfg[cfgAccessToken] = token.token.AccessToken
	newCfg[cfgRefreshToken] = token.token.RefreshToken
	newCfg[cfgEnvironment] = token.environment
	newCfg[cfgClientID] = token.clientID
	newCfg[cfgTenantID] = token.tenantID
	newCfg[cfgApiserverID] = token.apiserverID
	newCfg[cfgExpiresIn] = string(token.token.ExpiresIn)
	newCfg[cfgExpiresOn] = string(token.token.ExpiresOn)

	err := ts.persister.Persist(newCfg)
	if err != nil {
		return fmt.Errorf("persisting the configuration: %v", err)
	}
	ts.cfg = newCfg
	return nil
}

func (ts *azureTokenSource) refreshToken(token *azureToken) (*azureToken, error) {
	env, err := azure.EnvironmentFromName(token.environment)
	if err != nil {
		return nil, err
	}

	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, token.tenantID)
	if err != nil {
		return nil, fmt.Errorf("building the OAuth configuration for token refresh: %v", err)
	}

	callback := func(t adal.Token) error {
		return nil
	}
	spt, err := adal.NewServicePrincipalTokenFromManualToken(
		*oauthConfig,
		token.clientID,
		token.apiserverID,
		token.token,
		callback)
	if err != nil {
		return nil, fmt.Errorf("creating new service principal for token refresh: %v", err)
	}

	if err := spt.Refresh(); err != nil {
		return nil, fmt.Errorf("refreshing token: %v", err)
	}

	return &azureToken{
		token:       spt.Token(),
		environment: token.environment,
		clientID:    token.clientID,
		tenantID:    token.tenantID,
		apiserverID: token.apiserverID,
	}, nil
}

type azureTokenSourceDeviceCode struct {
	environment azure.Environment
	clientID    string
	tenantID    string
	apiserverID string
}

func newAzureTokenSourceDeviceCode(environment azure.Environment, clientID string, tenantID string, apiserverID string) (tokenSource, error) {
	if clientID == "" {
		return nil, errors.New("client-id is empty")
	}
	if tenantID == "" {
		return nil, errors.New("tenant-id is empty")
	}
	if apiserverID == "" {
		return nil, errors.New("apiserver-id is empty")
	}
	return &azureTokenSourceDeviceCode{
		environment: environment,
		clientID:    clientID,
		tenantID:    tenantID,
		apiserverID: apiserverID,
	}, nil
}

func (ts *azureTokenSourceDeviceCode) Token() (*azureToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(ts.environment.ActiveDirectoryEndpoint, ts.tenantID)
	if err != nil {
		return nil, fmt.Errorf("building the OAuth configuration for device code authentication: %v", err)
	}
	client := &autorest.Client{}
	deviceCode, err := adal.InitiateDeviceAuth(client, *oauthConfig, ts.clientID, ts.apiserverID)
	if err != nil {
		return nil, fmt.Errorf("initialing the device code authentication: %v", err)
	}

	_, err = fmt.Fprintln(os.Stderr, *deviceCode.Message)
	if err != nil {
		return nil, fmt.Errorf("prompting the device code message: %v", err)
	}

	token, err := adal.WaitForUserCompletion(client, deviceCode)
	if err != nil {
		return nil, fmt.Errorf("waiting for device code authentication to complete: %v", err)
	}

	return &azureToken{
		token:       *token,
		environment: ts.environment.Name,
		clientID:    ts.clientID,
		tenantID:    ts.tenantID,
		apiserverID: ts.apiserverID,
	}, nil
}
