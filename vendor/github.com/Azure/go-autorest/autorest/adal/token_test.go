package adal

// Copyright 2017 Microsoft Corporation
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/date"
	"github.com/Azure/go-autorest/autorest/mocks"
)

const (
	defaultFormData       = "client_id=id&client_secret=secret&grant_type=client_credentials&resource=resource"
	defaultManualFormData = "client_id=id&grant_type=refresh_token&refresh_token=refreshtoken&resource=resource"
)

func TestTokenExpires(t *testing.T) {
	tt := time.Now().Add(5 * time.Second)
	tk := newTokenExpiresAt(tt)

	if tk.Expires().Equal(tt) {
		t.Fatalf("adal: Token#Expires miscalculated expiration time -- received %v, expected %v", tk.Expires(), tt)
	}
}

func TestTokenIsExpired(t *testing.T) {
	tk := newTokenExpiresAt(time.Now().Add(-5 * time.Second))

	if !tk.IsExpired() {
		t.Fatalf("adal: Token#IsExpired failed to mark a stale token as expired -- now %v, token expires at %v",
			time.Now().UTC(), tk.Expires())
	}
}

func TestTokenIsExpiredUninitialized(t *testing.T) {
	tk := &Token{}

	if !tk.IsExpired() {
		t.Fatalf("adal: An uninitialized Token failed to mark itself as expired (expiration time %v)", tk.Expires())
	}
}

func TestTokenIsNoExpired(t *testing.T) {
	tk := newTokenExpiresAt(time.Now().Add(1000 * time.Second))

	if tk.IsExpired() {
		t.Fatalf("adal: Token marked a fresh token as expired -- now %v, token expires at %v", time.Now().UTC(), tk.Expires())
	}
}

func TestTokenWillExpireIn(t *testing.T) {
	d := 5 * time.Second
	tk := newTokenExpiresIn(d)

	if !tk.WillExpireIn(d) {
		t.Fatal("adal: Token#WillExpireIn mismeasured expiration time")
	}
}

func TestServicePrincipalTokenSetAutoRefresh(t *testing.T) {
	spt := newServicePrincipalToken()

	if !spt.inner.AutoRefresh {
		t.Fatal("adal: ServicePrincipalToken did not default to automatic token refreshing")
	}

	spt.SetAutoRefresh(false)
	if spt.inner.AutoRefresh {
		t.Fatal("adal: ServicePrincipalToken#SetAutoRefresh did not disable automatic token refreshing")
	}
}

func TestServicePrincipalTokenSetRefreshWithin(t *testing.T) {
	spt := newServicePrincipalToken()

	if spt.inner.RefreshWithin != defaultRefresh {
		t.Fatal("adal: ServicePrincipalToken did not correctly set the default refresh interval")
	}

	spt.SetRefreshWithin(2 * defaultRefresh)
	if spt.inner.RefreshWithin != 2*defaultRefresh {
		t.Fatal("adal: ServicePrincipalToken#SetRefreshWithin did not set the refresh interval")
	}
}

func TestServicePrincipalTokenSetSender(t *testing.T) {
	spt := newServicePrincipalToken()

	c := &http.Client{}
	spt.SetSender(c)
	if !reflect.DeepEqual(c, spt.sender) {
		t.Fatal("adal: ServicePrincipalToken#SetSender did not set the sender")
	}
}

func TestServicePrincipalTokenRefreshUsesPOST(t *testing.T) {
	spt := newServicePrincipalToken()

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					if r.Method != "POST" {
						t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set HTTP method -- expected %v, received %v", "POST", r.Method)
					}
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}

	if body.IsOpen() {
		t.Fatalf("the response was not closed!")
	}
}

func TestServicePrincipalTokenFromMSIRefreshUsesGET(t *testing.T) {
	resource := "https://resource"
	cb := func(token Token) error { return nil }

	spt, err := NewServicePrincipalTokenFromMSI("http://msiendpoint/", resource, cb)
	if err != nil {
		t.Fatalf("Failed to get MSI SPT: %v", err)
	}

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					if r.Method != "GET" {
						t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set HTTP method -- expected %v, received %v", "GET", r.Method)
					}
					if h := r.Header.Get("Metadata"); h != "true" {
						t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set Metadata header for MSI")
					}
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err = spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}

	if body.IsOpen() {
		t.Fatalf("the response was not closed!")
	}
}

func TestServicePrincipalTokenFromMSIRefreshCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	endpoint, _ := GetMSIVMEndpoint()

	spt, err := NewServicePrincipalTokenFromMSI(endpoint, "https://resource")
	if err != nil {
		t.Fatalf("Failed to get MSI SPT: %v", err)
	}

	c := mocks.NewSender()
	c.AppendAndRepeatResponse(mocks.NewResponseWithStatus("Internal server error", http.StatusInternalServerError), 5)

	var wg sync.WaitGroup
	wg.Add(1)
	start := time.Now()
	end := time.Now()

	go func() {
		spt.SetSender(c)
		err = spt.RefreshWithContext(ctx)
		end = time.Now()
		wg.Done()
	}()

	cancel()
	wg.Wait()
	time.Sleep(5 * time.Millisecond)

	if end.Sub(start) >= time.Second {
		t.Fatalf("TestServicePrincipalTokenFromMSIRefreshCancel failed to cancel")
	}
}

func TestServicePrincipalTokenRefreshSetsMimeType(t *testing.T) {
	spt := newServicePrincipalToken()

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					if r.Header.Get(http.CanonicalHeaderKey("Content-Type")) != "application/x-www-form-urlencoded" {
						t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set Content-Type -- expected %v, received %v",
							"application/x-form-urlencoded",
							r.Header.Get(http.CanonicalHeaderKey("Content-Type")))
					}
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}
}

func TestServicePrincipalTokenRefreshSetsURL(t *testing.T) {
	spt := newServicePrincipalToken()

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					if r.URL.String() != TestOAuthConfig.TokenEndpoint.String() {
						t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set the URL -- expected %v, received %v",
							TestOAuthConfig.TokenEndpoint, r.URL)
					}
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}
}

func testServicePrincipalTokenRefreshSetsBody(t *testing.T, spt *ServicePrincipalToken, f func(*testing.T, []byte)) {
	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					b, err := ioutil.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("adal: Failed to read body of Service Principal token request (%v)", err)
					}
					f(t, b)
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}
}

func TestServicePrincipalTokenManualRefreshSetsBody(t *testing.T) {
	sptManual := newServicePrincipalTokenManual()
	testServicePrincipalTokenRefreshSetsBody(t, sptManual, func(t *testing.T, b []byte) {
		if string(b) != defaultManualFormData {
			t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set the HTTP Request Body -- expected %v, received %v",
				defaultManualFormData, string(b))
		}
	})
}

func TestServicePrincipalTokenCertficateRefreshSetsBody(t *testing.T) {
	sptCert := newServicePrincipalTokenCertificate(t)
	testServicePrincipalTokenRefreshSetsBody(t, sptCert, func(t *testing.T, b []byte) {
		body := string(b)

		values, _ := url.ParseQuery(body)
		if values["client_assertion_type"][0] != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" ||
			values["client_id"][0] != "id" ||
			values["grant_type"][0] != "client_credentials" ||
			values["resource"][0] != "resource" {
			t.Fatalf("adal: ServicePrincipalTokenCertificate#Refresh did not correctly set the HTTP Request Body.")
		}
	})
}

func TestServicePrincipalTokenUsernamePasswordRefreshSetsBody(t *testing.T) {
	spt := newServicePrincipalTokenUsernamePassword(t)
	testServicePrincipalTokenRefreshSetsBody(t, spt, func(t *testing.T, b []byte) {
		body := string(b)

		values, _ := url.ParseQuery(body)
		if values["client_id"][0] != "id" ||
			values["grant_type"][0] != "password" ||
			values["username"][0] != "username" ||
			values["password"][0] != "password" ||
			values["resource"][0] != "resource" {
			t.Fatalf("adal: ServicePrincipalTokenUsernamePassword#Refresh did not correctly set the HTTP Request Body.")
		}
	})
}

func TestServicePrincipalTokenAuthorizationCodeRefreshSetsBody(t *testing.T) {
	spt := newServicePrincipalTokenAuthorizationCode(t)
	testServicePrincipalTokenRefreshSetsBody(t, spt, func(t *testing.T, b []byte) {
		body := string(b)

		values, _ := url.ParseQuery(body)
		if values["client_id"][0] != "id" ||
			values["grant_type"][0] != OAuthGrantTypeAuthorizationCode ||
			values["code"][0] != "code" ||
			values["client_secret"][0] != "clientSecret" ||
			values["redirect_uri"][0] != "http://redirectUri/getToken" ||
			values["resource"][0] != "resource" {
			t.Fatalf("adal: ServicePrincipalTokenAuthorizationCode#Refresh did not correctly set the HTTP Request Body.")
		}
	})
	testServicePrincipalTokenRefreshSetsBody(t, spt, func(t *testing.T, b []byte) {
		body := string(b)

		values, _ := url.ParseQuery(body)
		if values["client_id"][0] != "id" ||
			values["grant_type"][0] != OAuthGrantTypeRefreshToken ||
			values["code"][0] != "code" ||
			values["client_secret"][0] != "clientSecret" ||
			values["redirect_uri"][0] != "http://redirectUri/getToken" ||
			values["resource"][0] != "resource" {
			t.Fatalf("adal: ServicePrincipalTokenAuthorizationCode#Refresh did not correctly set the HTTP Request Body.")
		}
	})
}

func TestServicePrincipalTokenSecretRefreshSetsBody(t *testing.T) {
	spt := newServicePrincipalToken()
	testServicePrincipalTokenRefreshSetsBody(t, spt, func(t *testing.T, b []byte) {
		if string(b) != defaultFormData {
			t.Fatalf("adal: ServicePrincipalToken#Refresh did not correctly set the HTTP Request Body -- expected %v, received %v",
				defaultFormData, string(b))
		}

	})
}

func TestServicePrincipalTokenRefreshClosesRequestBody(t *testing.T) {
	spt := newServicePrincipalToken()

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}
	if resp.Body.(*mocks.Body).IsOpen() {
		t.Fatal("adal: ServicePrincipalToken#Refresh failed to close the HTTP Response Body")
	}
}

func TestServicePrincipalTokenRefreshRejectsResponsesWithStatusNotOK(t *testing.T) {
	spt := newServicePrincipalToken()

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusUnauthorized, "Unauthorized")

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err == nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh should reject a response with status != %d", http.StatusOK)
	}
}

func TestServicePrincipalTokenRefreshRejectsEmptyBody(t *testing.T) {
	spt := newServicePrincipalToken()

	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					return mocks.NewResponse(), nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.Refresh()
	if err == nil {
		t.Fatal("adal: ServicePrincipalToken#Refresh should reject an empty token")
	}
}

func TestServicePrincipalTokenRefreshPropagatesErrors(t *testing.T) {
	spt := newServicePrincipalToken()

	c := mocks.NewSender()
	c.SetError(fmt.Errorf("Faux Error"))
	spt.SetSender(c)

	err := spt.Refresh()
	if err == nil {
		t.Fatal("adal: Failed to propagate the request error")
	}
}

func TestServicePrincipalTokenRefreshReturnsErrorIfNotOk(t *testing.T) {
	spt := newServicePrincipalToken()

	c := mocks.NewSender()
	c.AppendResponse(mocks.NewResponseWithStatus("401 NotAuthorized", http.StatusUnauthorized))
	spt.SetSender(c)

	err := spt.Refresh()
	if err == nil {
		t.Fatalf("adal: Failed to return an when receiving a status code other than HTTP %d", http.StatusOK)
	}
}

func TestServicePrincipalTokenRefreshUnmarshals(t *testing.T) {
	spt := newServicePrincipalToken()

	expiresOn := strconv.Itoa(int(time.Now().Add(3600 * time.Second).Sub(date.UnixEpoch()).Seconds()))
	j := newTokenJSON(expiresOn, "resource")
	resp := mocks.NewResponseWithContent(j)
	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)

	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	} else if spt.inner.Token.AccessToken != "accessToken" ||
		spt.inner.Token.ExpiresIn != "3600" ||
		spt.inner.Token.ExpiresOn != expiresOn ||
		spt.inner.Token.NotBefore != expiresOn ||
		spt.inner.Token.Resource != "resource" ||
		spt.inner.Token.Type != "Bearer" {
		t.Fatalf("adal: ServicePrincipalToken#Refresh failed correctly unmarshal the JSON -- expected %v, received %v",
			j, *spt)
	}
}

func TestServicePrincipalTokenEnsureFreshRefreshes(t *testing.T) {
	spt := newServicePrincipalToken()
	expireToken(&spt.inner.Token)

	body := mocks.NewBody(newTokenJSON("test", "test"))
	resp := mocks.NewResponseWithBodyAndStatus(body, http.StatusOK, "OK")

	f := false
	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					f = true
					return resp, nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.EnsureFresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#EnsureFresh returned an unexpected error (%v)", err)
	}
	if !f {
		t.Fatal("adal: ServicePrincipalToken#EnsureFresh failed to call Refresh for stale token")
	}
}

func TestServicePrincipalTokenEnsureFreshFails(t *testing.T) {
	spt := newServicePrincipalToken()
	expireToken(&spt.inner.Token)

	c := mocks.NewSender()
	c.SetError(fmt.Errorf("some failure"))

	spt.SetSender(c)
	err := spt.EnsureFresh()
	if err == nil {
		t.Fatal("adal: ServicePrincipalToken#EnsureFresh didn't return an error")
	}
	if _, ok := err.(TokenRefreshError); !ok {
		t.Fatal("adal: ServicePrincipalToken#EnsureFresh didn't return a TokenRefreshError")
	}
}

func TestServicePrincipalTokenEnsureFreshSkipsIfFresh(t *testing.T) {
	spt := newServicePrincipalToken()
	setTokenToExpireIn(&spt.inner.Token, 1000*time.Second)

	f := false
	c := mocks.NewSender()
	s := DecorateSender(c,
		(func() SendDecorator {
			return func(s Sender) Sender {
				return SenderFunc(func(r *http.Request) (*http.Response, error) {
					f = true
					return mocks.NewResponse(), nil
				})
			}
		})())
	spt.SetSender(s)
	err := spt.EnsureFresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#EnsureFresh returned an unexpected error (%v)", err)
	}
	if f {
		t.Fatal("adal: ServicePrincipalToken#EnsureFresh invoked Refresh for fresh token")
	}
}

func TestRefreshCallback(t *testing.T) {
	callbackTriggered := false
	spt := newServicePrincipalToken(func(Token) error {
		callbackTriggered = true
		return nil
	})

	expiresOn := strconv.Itoa(int(time.Now().Add(3600 * time.Second).Sub(date.UnixEpoch()).Seconds()))

	sender := mocks.NewSender()
	j := newTokenJSON(expiresOn, "resource")
	sender.AppendResponse(mocks.NewResponseWithContent(j))
	spt.SetSender(sender)
	err := spt.Refresh()
	if err != nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh returned an unexpected error (%v)", err)
	}
	if !callbackTriggered {
		t.Fatalf("adal: RefreshCallback failed to trigger call callback")
	}
}

func TestRefreshCallbackErrorPropagates(t *testing.T) {
	errorText := "this is an error text"
	spt := newServicePrincipalToken(func(Token) error {
		return fmt.Errorf(errorText)
	})

	expiresOn := strconv.Itoa(int(time.Now().Add(3600 * time.Second).Sub(date.UnixEpoch()).Seconds()))

	sender := mocks.NewSender()
	j := newTokenJSON(expiresOn, "resource")
	sender.AppendResponse(mocks.NewResponseWithContent(j))
	spt.SetSender(sender)
	err := spt.Refresh()

	if err == nil || !strings.Contains(err.Error(), errorText) {
		t.Fatalf("adal: RefreshCallback failed to propagate error")
	}
}

// This demonstrates the danger of manual token without a refresh token
func TestServicePrincipalTokenManualRefreshFailsWithoutRefresh(t *testing.T) {
	spt := newServicePrincipalTokenManual()
	spt.inner.Token.RefreshToken = ""
	err := spt.Refresh()
	if err == nil {
		t.Fatalf("adal: ServicePrincipalToken#Refresh should have failed with a ManualTokenSecret without a refresh token")
	}
}

func TestNewServicePrincipalTokenFromMSI(t *testing.T) {
	resource := "https://resource"
	cb := func(token Token) error { return nil }

	spt, err := NewServicePrincipalTokenFromMSI("http://msiendpoint/", resource, cb)
	if err != nil {
		t.Fatalf("Failed to get MSI SPT: %v", err)
	}

	// check some of the SPT fields
	if _, ok := spt.inner.Secret.(*ServicePrincipalMSISecret); !ok {
		t.Fatal("SPT secret was not of MSI type")
	}

	if spt.inner.Resource != resource {
		t.Fatal("SPT came back with incorrect resource")
	}

	if len(spt.refreshCallbacks) != 1 {
		t.Fatal("SPT had incorrect refresh callbacks.")
	}
}

func TestNewServicePrincipalTokenFromMSIWithUserAssignedID(t *testing.T) {
	resource := "https://resource"
	userID := "abc123"
	cb := func(token Token) error { return nil }

	spt, err := NewServicePrincipalTokenFromMSIWithUserAssignedID("http://msiendpoint/", resource, userID, cb)
	if err != nil {
		t.Fatalf("Failed to get MSI SPT: %v", err)
	}

	// check some of the SPT fields
	if _, ok := spt.inner.Secret.(*ServicePrincipalMSISecret); !ok {
		t.Fatal("SPT secret was not of MSI type")
	}

	if spt.inner.Resource != resource {
		t.Fatal("SPT came back with incorrect resource")
	}

	if len(spt.refreshCallbacks) != 1 {
		t.Fatal("SPT had incorrect refresh callbacks.")
	}

	if spt.inner.ClientID != userID {
		t.Fatal("SPT had incorrect client ID")
	}
}

func TestNewServicePrincipalTokenFromManualTokenSecret(t *testing.T) {
	token := newToken()
	secret := &ServicePrincipalAuthorizationCodeSecret{
		ClientSecret:      "clientSecret",
		AuthorizationCode: "code123",
		RedirectURI:       "redirect",
	}

	spt, err := NewServicePrincipalTokenFromManualTokenSecret(TestOAuthConfig, "id", "resource", *token, secret, nil)
	if err != nil {
		t.Fatalf("Failed creating new SPT: %s", err)
	}

	if !reflect.DeepEqual(*token, spt.inner.Token) {
		t.Fatalf("Tokens do not match: %s, %s", *token, spt.inner.Token)
	}

	if !reflect.DeepEqual(secret, spt.inner.Secret) {
		t.Fatalf("Secrets do not match: %s, %s", secret, spt.inner.Secret)
	}

}

func TestGetVMEndpoint(t *testing.T) {
	endpoint, err := GetMSIVMEndpoint()
	if err != nil {
		t.Fatal("Coudn't get VM endpoint")
	}

	if endpoint != msiEndpoint {
		t.Fatal("Didn't get correct endpoint")
	}
}

func TestMarshalServicePrincipalNoSecret(t *testing.T) {
	spt := newServicePrincipalTokenManual()
	b, err := json.Marshal(spt)
	if err != nil {
		t.Fatalf("failed to marshal token: %+v", err)
	}
	var spt2 *ServicePrincipalToken
	err = json.Unmarshal(b, &spt2)
	if err != nil {
		t.Fatalf("failed to unmarshal token: %+v", err)
	}
	if !reflect.DeepEqual(spt, spt2) {
		t.Fatal("tokens don't match")
	}
}

func TestMarshalServicePrincipalTokenSecret(t *testing.T) {
	spt := newServicePrincipalToken()
	b, err := json.Marshal(spt)
	if err != nil {
		t.Fatalf("failed to marshal token: %+v", err)
	}
	var spt2 *ServicePrincipalToken
	err = json.Unmarshal(b, &spt2)
	if err != nil {
		t.Fatalf("failed to unmarshal token: %+v", err)
	}
	if !reflect.DeepEqual(spt, spt2) {
		t.Fatal("tokens don't match")
	}
}

func TestMarshalServicePrincipalCertificateSecret(t *testing.T) {
	spt := newServicePrincipalTokenCertificate(t)
	b, err := json.Marshal(spt)
	if err == nil {
		t.Fatal("expected error when marshalling certificate token")
	}
	var spt2 *ServicePrincipalToken
	err = json.Unmarshal(b, &spt2)
	if err == nil {
		t.Fatal("expected error when unmarshalling certificate token")
	}
}

func TestMarshalServicePrincipalMSISecret(t *testing.T) {
	spt, err := newServicePrincipalTokenFromMSI("http://msiendpoint/", "https://resource", nil)
	if err != nil {
		t.Fatalf("failed to get MSI SPT: %+v", err)
	}
	b, err := json.Marshal(spt)
	if err == nil {
		t.Fatal("expected error when marshalling MSI token")
	}
	var spt2 *ServicePrincipalToken
	err = json.Unmarshal(b, &spt2)
	if err == nil {
		t.Fatal("expected error when unmarshalling MSI token")
	}
}

func TestMarshalServicePrincipalUsernamePasswordSecret(t *testing.T) {
	spt := newServicePrincipalTokenUsernamePassword(t)
	b, err := json.Marshal(spt)
	if err != nil {
		t.Fatalf("failed to marshal token: %+v", err)
	}
	var spt2 *ServicePrincipalToken
	err = json.Unmarshal(b, &spt2)
	if err != nil {
		t.Fatalf("failed to unmarshal token: %+v", err)
	}
	if !reflect.DeepEqual(spt, spt2) {
		t.Fatal("tokens don't match")
	}
}

func TestMarshalServicePrincipalAuthorizationCodeSecret(t *testing.T) {
	spt := newServicePrincipalTokenAuthorizationCode(t)
	b, err := json.Marshal(spt)
	if err != nil {
		t.Fatalf("failed to marshal token: %+v", err)
	}
	var spt2 *ServicePrincipalToken
	err = json.Unmarshal(b, &spt2)
	if err != nil {
		t.Fatalf("failed to unmarshal token: %+v", err)
	}
	if !reflect.DeepEqual(spt, spt2) {
		t.Fatal("tokens don't match")
	}
}

func TestMarshalInnerToken(t *testing.T) {
	spt := newServicePrincipalTokenManual()
	tokenJSON, err := spt.MarshalTokenJSON()
	if err != nil {
		t.Fatalf("failed to marshal token: %+v", err)
	}

	testToken := newToken()
	testToken.RefreshToken = "refreshtoken"

	testTokenJSON, err := json.Marshal(testToken)
	if err != nil {
		t.Fatalf("failed to marshal test token: %+v", err)
	}

	if !reflect.DeepEqual(tokenJSON, testTokenJSON) {
		t.Fatalf("tokens don't match: %s, %s", tokenJSON, testTokenJSON)
	}

	var t1 *Token
	err = json.Unmarshal(tokenJSON, &t1)
	if err != nil {
		t.Fatalf("failed to unmarshal token: %+v", err)
	}

	if !reflect.DeepEqual(t1, testToken) {
		t.Fatalf("tokens don't match: %s, %s", t1, testToken)
	}
}

func newToken() *Token {
	return &Token{
		AccessToken: "ASECRETVALUE",
		Resource:    "https://azure.microsoft.com/",
		Type:        "Bearer",
	}
}

func newTokenJSON(expiresOn string, resource string) string {
	return fmt.Sprintf(`{
		"access_token" : "accessToken",
		"expires_in"   : "3600",
		"expires_on"   : "%s",
		"not_before"   : "%s",
		"resource"     : "%s",
		"token_type"   : "Bearer",
		"refresh_token": "ABC123"
		}`,
		expiresOn, expiresOn, resource)
}

func newTokenExpiresIn(expireIn time.Duration) *Token {
	return setTokenToExpireIn(newToken(), expireIn)
}

func newTokenExpiresAt(expireAt time.Time) *Token {
	return setTokenToExpireAt(newToken(), expireAt)
}

func expireToken(t *Token) *Token {
	return setTokenToExpireIn(t, 0)
}

func setTokenToExpireAt(t *Token, expireAt time.Time) *Token {
	t.ExpiresIn = "3600"
	t.ExpiresOn = strconv.Itoa(int(expireAt.Sub(date.UnixEpoch()).Seconds()))
	t.NotBefore = t.ExpiresOn
	return t
}

func setTokenToExpireIn(t *Token, expireIn time.Duration) *Token {
	return setTokenToExpireAt(t, time.Now().Add(expireIn))
}

func newServicePrincipalToken(callbacks ...TokenRefreshCallback) *ServicePrincipalToken {
	spt, _ := NewServicePrincipalToken(TestOAuthConfig, "id", "secret", "resource", callbacks...)
	return spt
}

func newServicePrincipalTokenManual() *ServicePrincipalToken {
	token := newToken()
	token.RefreshToken = "refreshtoken"
	spt, _ := NewServicePrincipalTokenFromManualToken(TestOAuthConfig, "id", "resource", *token)
	return spt
}

func newServicePrincipalTokenCertificate(t *testing.T) *ServicePrincipalToken {
	template := x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               pkix.Name{CommonName: "test"},
		BasicConstraintsValid: true,
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	certificateBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(certificateBytes)
	if err != nil {
		t.Fatal(err)
	}

	spt, _ := NewServicePrincipalTokenFromCertificate(TestOAuthConfig, "id", certificate, privateKey, "resource")
	return spt
}

func newServicePrincipalTokenUsernamePassword(t *testing.T) *ServicePrincipalToken {
	spt, _ := NewServicePrincipalTokenFromUsernamePassword(TestOAuthConfig, "id", "username", "password", "resource")
	return spt
}

func newServicePrincipalTokenAuthorizationCode(t *testing.T) *ServicePrincipalToken {
	spt, _ := NewServicePrincipalTokenFromAuthorizationCode(TestOAuthConfig, "id", "clientSecret", "code", "http://redirectUri/getToken", "resource")
	return spt
}
