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

package openstack

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/util/net"
	restclient "k8s.io/client-go/rest"
)

func init() {
	if err := restclient.RegisterAuthProviderPlugin("openstack", newOpenstackAuthProvider); err != nil {
		klog.Fatalf("Failed to register openstack auth plugin: %s", err)
	}
}

// DefaultTTLDuration is the time before a token gets expired.
const DefaultTTLDuration = 10 * time.Minute

// openstackAuthProvider is an authprovider for openstack. this provider reads
// the environment variables to determine the client identity, and generates a
// token which will be inserted into the request header later.
type openstackAuthProvider struct {
	ttl         time.Duration
	tokenGetter TokenGetter
}

// TokenGetter returns a bearer token that can be inserted into request.
type TokenGetter interface {
	Token() (string, error)
}

type tokenGetter struct {
	authOpt *gophercloud.AuthOptions
}

// Token creates a token by authenticate with keystone.
func (t *tokenGetter) Token() (string, error) {
	var options gophercloud.AuthOptions
	var err error
	if t.authOpt == nil {
		// reads the config from the environment
		klog.V(4).Info("reading openstack config from the environment variables")
		options, err = openstack.AuthOptionsFromEnv()
		if err != nil {
			return "", fmt.Errorf("failed to read openstack env vars: %s", err)
		}
	} else {
		options = *t.authOpt
	}
	client, err := openstack.AuthenticatedClient(options)
	if err != nil {
		return "", fmt.Errorf("authentication failed: %s", err)
	}
	return client.TokenID, nil
}

// cachedGetter caches a token until it gets expired, after the expiration, it will
// generate another token and cache it.
type cachedGetter struct {
	mutex       sync.Mutex
	tokenGetter TokenGetter

	token string
	born  time.Time
	ttl   time.Duration
}

// Token returns the current available token, create a new one if expired.
func (c *cachedGetter) Token() (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var err error
	// no token or exceeds the TTL
	if c.token == "" || time.Since(c.born) > c.ttl {
		c.token, err = c.tokenGetter.Token()
		if err != nil {
			return "", fmt.Errorf("failed to get token: %s", err)
		}
		c.born = time.Now()
	}
	return c.token, nil
}

// tokenRoundTripper implements the RoundTripper interface: adding the bearer token
// into the request header.
type tokenRoundTripper struct {
	http.RoundTripper

	tokenGetter TokenGetter
}

var _ net.RoundTripperWrapper = &tokenRoundTripper{}

// RoundTrip adds the bearer token into the request.
func (t *tokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// if the authorization header already present, use it.
	if req.Header.Get("Authorization") != "" {
		return t.RoundTripper.RoundTrip(req)
	}

	token, err := t.tokenGetter.Token()
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		klog.V(4).Infof("failed to get token: %s", err)
	}

	return t.RoundTripper.RoundTrip(req)
}

func (t *tokenRoundTripper) WrappedRoundTripper() http.RoundTripper { return t.RoundTripper }

// newOpenstackAuthProvider creates an auth provider which works with openstack
// environment.
func newOpenstackAuthProvider(_ string, config map[string]string, persister restclient.AuthProviderConfigPersister) (restclient.AuthProvider, error) {
	var ttlDuration time.Duration
	var err error

	klog.Warningf("WARNING: in-tree openstack auth plugin is now deprecated. please use the \"client-keystone-auth\" kubectl/client-go credential plugin instead")
	ttl, found := config["ttl"]
	if !found {
		ttlDuration = DefaultTTLDuration
		// persist to config
		config["ttl"] = ttlDuration.String()
		if err = persister.Persist(config); err != nil {
			return nil, fmt.Errorf("failed to persist config: %s", err)
		}
	} else {
		ttlDuration, err = time.ParseDuration(ttl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ttl config: %s", err)
		}
	}

	authOpt := gophercloud.AuthOptions{
		IdentityEndpoint: config["identityEndpoint"],
		Username:         config["username"],
		Password:         config["password"],
		DomainName:       config["name"],
		TenantID:         config["tenantId"],
		TenantName:       config["tenantName"],
	}

	getter := tokenGetter{}
	// not empty
	if (authOpt != gophercloud.AuthOptions{}) {
		if len(authOpt.IdentityEndpoint) == 0 {
			return nil, fmt.Errorf("empty %q in the config for openstack auth provider", "identityEndpoint")
		}
		getter.authOpt = &authOpt
	}

	return &openstackAuthProvider{
		ttl:         ttlDuration,
		tokenGetter: &getter,
	}, nil
}

func (oap *openstackAuthProvider) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &tokenRoundTripper{
		RoundTripper: rt,
		tokenGetter: &cachedGetter{
			tokenGetter: oap.tokenGetter,
			ttl:         oap.ttl,
		},
	}
}

func (oap *openstackAuthProvider) Login() error { return nil }
