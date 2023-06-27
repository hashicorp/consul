// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kubeauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/square/go-jose.v2/jwt"
	authv1 "k8s.io/api/authentication/v1"
	client_metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	client_authv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	client_corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	client_rest "k8s.io/client-go/rest"
	cert "k8s.io/client-go/util/cert"

	cleanhttp "github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
)

func init() {
	// register this as an available auth method type
	authmethod.Register("kubernetes", func(logger hclog.Logger, method *structs.ACLAuthMethod) (authmethod.Validator, error) {
		v, err := NewValidator(method)
		if err != nil {
			return nil, err
		}
		return v, nil
	})
}

const (
	serviceAccountNamespaceField = "serviceaccount.namespace"
	serviceAccountNameField      = "serviceaccount.name"
	serviceAccountUIDField       = "serviceaccount.uid"

	serviceAccountServiceNameAnnotation = "consul.hashicorp.com/service-name"
)

type Config struct {
	// Host must be a host string, a host:port pair, or a URL to the base of
	// the Kubernetes API server.
	Host string `json:",omitempty"`

	// PEM encoded CA cert for use by the TLS client used to talk with the
	// Kubernetes API. Every line must end with a newline: \n
	CACert string `json:",omitempty"`

	// A service account JWT used to access the TokenReview API to validate
	// other JWTs during login. It also must be able to read ServiceAccount
	// annotations.
	ServiceAccountJWT string `json:",omitempty"`

	enterpriseConfig `mapstructure:",squash"`
}

// Validator is the wrapper around the relevant portions of the Kubernetes API
// that also conforms to the authmethod.Validator interface.
type Validator struct {
	name     string
	config   *Config
	saGetter client_corev1.ServiceAccountsGetter
	trGetter client_authv1.TokenReviewsGetter
}

func NewValidator(method *structs.ACLAuthMethod) (*Validator, error) {
	if method.Type != "kubernetes" {
		return nil, fmt.Errorf("%q is not a kubernetes auth method", method.Name)
	}

	var config Config
	if err := authmethod.ParseConfig(method.Config, &config); err != nil {
		return nil, err
	}

	if config.Host == "" {
		return nil, fmt.Errorf("Config.Host is required")
	}

	if config.CACert == "" {
		return nil, fmt.Errorf("Config.CACert is required")
	}
	if _, err := cert.ParseCertsPEM([]byte(config.CACert)); err != nil {
		return nil, fmt.Errorf("error parsing kubernetes ca cert: %v", err)
	}

	// This is the bearer token we give the apiserver to use the API.
	if config.ServiceAccountJWT == "" {
		return nil, fmt.Errorf("Config.ServiceAccountJWT is required")
	}
	if _, err := jwt.ParseSigned(config.ServiceAccountJWT); err != nil {
		return nil, fmt.Errorf("Config.ServiceAccountJWT is not a valid JWT: %v", err)
	}

	if err := enterpriseValidation(method, &config); err != nil {
		return nil, err
	}

	transport := cleanhttp.DefaultTransport()
	client, err := k8s.NewForConfig(&client_rest.Config{
		Host:        config.Host,
		BearerToken: config.ServiceAccountJWT,
		Dial:        transport.DialContext,
		TLSClientConfig: client_rest.TLSClientConfig{
			CAData: []byte(config.CACert),
		},
		ContentConfig: client_rest.ContentConfig{
			ContentType: "application/json",
		},
	})
	if err != nil {
		return nil, err
	}

	return &Validator{
		name:     method.Name,
		config:   &config,
		saGetter: client.CoreV1(),
		trGetter: client.AuthenticationV1(),
	}, nil
}

func (v *Validator) Name() string { return v.name }

func (v *Validator) Stop() {}

func (v *Validator) ValidateLogin(ctx context.Context, loginToken string) (*authmethod.Identity, error) {
	if _, err := jwt.ParseSigned(loginToken); err != nil {
		return nil, fmt.Errorf("failed to parse and validate JWT: %v", err)
	}

	// Check TokenReview for the bulk of the work.
	trResp, err := v.trGetter.TokenReviews().Create(ctx, &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: loginToken,
		},
	}, client_metav1.CreateOptions{})

	if err != nil {
		return nil, err
	} else if trResp.Status.Error != "" {
		return nil, fmt.Errorf("lookup failed: %s", trResp.Status.Error)
	}

	if !trResp.Status.Authenticated {
		return nil, errors.New("lookup failed: service account jwt not valid")
	}

	// The username is of format: system:serviceaccount:(NAMESPACE):(SERVICEACCOUNT)
	parts := strings.Split(trResp.Status.User.Username, ":")
	if len(parts) != 4 {
		return nil, errors.New("lookup failed: unexpected username format")
	}

	// Validate the user that comes back from token review is a service account
	if parts[0] != "system" || parts[1] != "serviceaccount" {
		return nil, errors.New("lookup failed: username returned is not a service account")
	}

	var (
		saNamespace = parts[2]
		saName      = parts[3]
		saUID       = trResp.Status.User.UID
	)

	// Check to see  if there is an override name on the ServiceAccount object.
	sa, err := v.saGetter.ServiceAccounts(saNamespace).Get(ctx, saName, client_metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("annotation lookup failed: %v", err)
	}

	annotations := sa.GetObjectMeta().GetAnnotations()
	if serviceNameOverride, ok := annotations[serviceAccountServiceNameAnnotation]; ok {
		saName = serviceNameOverride
	}

	fields := map[string]string{
		serviceAccountNamespaceField: saNamespace,
		serviceAccountNameField:      saName,
		serviceAccountUIDField:       saUID,
	}

	id := v.NewIdentity()
	id.SelectableFields = &k8sFieldDetails{
		ServiceAccount: k8sFieldDetailsServiceAccount{
			Namespace: fields[serviceAccountNamespaceField],
			Name:      fields[serviceAccountNameField],
			UID:       fields[serviceAccountUIDField],
		},
	}
	for k, val := range fields {
		id.ProjectedVars[k] = val
	}
	id.EnterpriseMeta = v.k8sEntMetaFromFields(fields)

	return id, nil
}

func (v *Validator) NewIdentity() *authmethod.Identity {
	id := &authmethod.Identity{
		SelectableFields: &k8sFieldDetails{},
		ProjectedVars:    map[string]string{},
	}
	for _, f := range availableFields {
		id.ProjectedVars[f] = ""
	}
	return id
}

var availableFields = []string{
	serviceAccountNamespaceField,
	serviceAccountNameField,
	serviceAccountUIDField,
}

type k8sFieldDetails struct {
	ServiceAccount k8sFieldDetailsServiceAccount `bexpr:"serviceaccount"`
}

type k8sFieldDetailsServiceAccount struct {
	Namespace string `bexpr:"namespace"`
	Name      string `bexpr:"name"`
	UID       string `bexpr:"uid"`
}
