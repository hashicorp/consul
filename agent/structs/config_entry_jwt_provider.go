package structs

import (
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/consul/acl"
)

type JWTProviderConfigEntry struct {
	// Kind is the kind of configuration entry and must be "jwt-provider".
	Kind string

	// Name is the name of the provider being configured.
	Name string

	// JSONWebKeySet defines a JSON Web Key Set, its location on disk, or the
	// means with which to fetch a key set from a remote server.
	JSONWebKeySet *JSONWebKeySet `json:",omitempty"`

	// Issuer is the entity that must have issued the JWT.
	// This value must match the "iss" claim of the token.
	Issuer string

	// Audiences is the set of audiences the JWT is allowed to access.
	// If specified, all JWTs verified with this provider must address
	// at least one of these to be considered valid.
	Audiences []string

	// Locations where the JWT will be present in requests.
	// Envoy will check all of these locations to extract a JWT.
	// If no locations are specified Envoy will default to:
	// 1. Authorization header with Bearer schema:
	//    "Authorization: Bearer <token>"
	// 2. access_token query parameter.
	Locations []*JWTLocation `json:",omitempty"`

	// Forwarding defines rules for forwarding verified JWTs to the backend.
	Forwarding *JWTForwardingConfig `json:",omitempty"`

	// ClockSkewSeconds specifies the maximum allowable time difference
	// from clock skew when validating the "exp" (Expiration) and "nbf"
	// (Not Before) claims.
	//
	// Default value is 30 seconds.
	ClockSkewSeconds int

	// CacheConfig defines configuration for caching the validation
	// result for previously seen JWTs. Caching results can speed up
	// verification when individual tokens are expected to be handled
	// multiple times.
	CacheConfig *JWTCacheConfig `json:",omitempty"`

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

// JWTLocation is a location where the JWT could be present in requests.
//
// Only one of Header, QueryParam, or Cookie can be specified.
type JWTLocation struct {
	// Header defines how to extract a JWT from an HTTP request header.
	Header *JWTLocationHeader

	// QueryParam defines how to extract a JWT from an HTTP request
	// query parameter.
	QueryParam *JWTLocationQueryParam

	// Cookie defines how to extract a JWT from an HTTP request cookie.
	Cookie *JWTLocationCookie
}

// JWTLocationHeader defines how to extract a JWT from an HTTP
// request header.
type JWTLocationHeader struct {
	// Name is the name of the header containing the token.
	Name string

	// ValuePrefix is an optional prefix that precedes the token in the
	// header value.
	// For example, "Bearer " is a standard value prefix for a header named
	// "Authorization", but the prefix is not part of the token itself:
	// "Authorization: Bearer <token>"
	ValuePrefix string

	// Forward defines whether the header with the JWT should be
	// forwarded after the token has been verified. If false, the
	// header will not be forwarded to the backend.
	//
	// Default value is false.
	Forward bool
}

// JWTLocationQueryParam defines how to extract a JWT from an HTTP request query parameter.
type JWTLocationQueryParam struct {
	// Name is the name of the query param containing the token.
	Name string
}

// JWTLocationCookie defines how to extract a JWT from an HTTP request cookie.
type JWTLocationCookie struct {
	// Name is the name of the cookie containing the token.
	Name string
}

type JWTForwardingConfig struct {
	// HeaderName is a header name to use when forwarding a verified
	// JWT to the backend. The verified JWT could have been extracted
	// from any location (query param, header, or cookie).
	//
	// The header value will be base64-URL-encoded, and will not be
	// padded unless PadForwardPayloadHeader is true.
	HeaderName string

	// PadForwardPayloadHeader determines whether padding should be added
	// to the base64 encoded token forwarded with ForwardPayloadHeader.
	//
	// Default value is false.
	PadForwardPayloadHeader bool
}

// JSONWebKeySet defines a key set, its location on disk, or the
// means with which to fetch a key set from a remote server.
//
// Exactly one of Local or Remote must be specified.
type JSONWebKeySet struct {
	// Local specifies a local source for the key set.
	Local *LocalJWKS

	// Remote specifies how to fetch a key set from a remote server.
	Remote *RemoteJWKS
}

// LocalJWKS specifies a location for a local JWKS.
//
// Only one of String and Filename can be specified.
type LocalJWKS struct {
	// String contains a base64 encoded JWKS.
	String string

	// Filename configures a location on disk where the JWKS can be
	// found. If specified, the file must be present on the disk of ALL
	// proxies with intentions referencing this provider.
	Filename string
}

// RemoteJWKS specifies how to fetch a JWKS from a remote server.
type RemoteJWKS struct {
	// URI is the URI of the server to query for the JWKS.
	URI string

	// RequestTimeoutMs is the number of milliseconds to
	// time out when making a request for the JWKS.
	RequestTimeoutMs int

	// CacheDuration is the duration after which cached keys
	// should be expired.
	//
	// Default value is 5 minutes.
	CacheDuration time.Duration

	// FetchAsynchronously indicates that the JWKS should be fetched
	// when a client request arrives. Client requests will be paused
	// until the JWKS is fetched.
	// If false, the proxy listener will wait for the JWKS to be
	// fetched before being activated.
	//
	// Default value is false.
	FetchAsynchronously bool

	// RetryPolicy defines a retry policy for fetching JWKS.
	//
	// There is no retry by default.
	RetryPolicy *JWKSRetryPolicy
}

type JWKSRetryPolicy struct {
	// NumRetries is the number of times to retry fetching the JWKS.
	// The retry strategy uses jittered exponential backoff with
	// a base interval of 1s and max of 10s.
	//
	// Default value is 0.
	NumRetries int
}

type JWTCacheConfig struct {
	// Size specifies the maximum number of JWT verification
	// results to cache.
	//
	// Defaults to 0, meaning that JWT caching is disabled.
	Size int
}

func (e *JWTProviderConfigEntry) GetKind() string {
	return JWTProvider
}
func (e *JWTProviderConfigEntry) GetName() string                        { return e.Name }
func (e *JWTProviderConfigEntry) GetMeta() map[string]string             { return e.Meta }
func (e *JWTProviderConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta { return &e.EnterpriseMeta }
func (e *JWTProviderConfigEntry) GetRaftIndex() *RaftIndex               { return &e.RaftIndex }
func (e *JWTProviderConfigEntry) GetJSONWebKeySet() *JSONWebKeySet       { return e.JSONWebKeySet }
func (e *JWTProviderConfigEntry) GetIssuer() string                      { return e.Issuer }
func (e *JWTProviderConfigEntry) GetAudiences() []string                 { return e.Audiences }
func (e *JWTProviderConfigEntry) GetLocations() []*JWTLocation           { return e.Locations }
func (e *JWTProviderConfigEntry) GetForwarding() *JWTForwardingConfig    { return e.Forwarding }
func (e *JWTProviderConfigEntry) GetClockSkewSeconds() int               { return e.ClockSkewSeconds }
func (e *JWTProviderConfigEntry) GetCacheConfig() *JWTCacheConfig        { return e.CacheConfig }

func (e *JWTProviderConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *JWTProviderConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (jwks *JSONWebKeySet) Validate() error {
	hasLocalKeySet := jwks.Local != nil
	hasRemoteKeySet := jwks.Remote != nil

	if (hasLocalKeySet && hasRemoteKeySet) || !(hasLocalKeySet || hasRemoteKeySet) {
		return fmt.Errorf("must specify exactly one of Local or Remote JSON Web key set")
	}

	// figure out if i can do something like
	// keySet := jwks.Remote || jwks.Local
	// keySet.Validate()
	if hasRemoteKeySet {
		if err := jwks.Remote.Validate(); err != nil {
			return err
		}
		return nil
	}

	if err := jwks.Local.Validate(); err != nil {
		return err
	}

	return nil
}

func (ks *LocalJWKS) Validate() error {
	hasFilename := ks.Filename != ""
	hasString := ks.String != ""

	if (hasFilename && hasString) || !(hasFilename || hasString) {
		return fmt.Errorf("must specify exactly one of String or filename for local keyset")
	}

	return nil
}

func (ks *RemoteJWKS) Validate() error {
	if ks.URI != "" {
		return fmt.Errorf("remote JWKS URI is required")
	}

	if _, err := url.ParseRequestURI(ks.URI); err != nil {
		return err
	}

	return nil
}

func (header *JWTLocationHeader) Validate() error {
	if header.Name == "" {
		return fmt.Errorf("JWT location header name must be specified")
	}
	return nil
}

func (cookieHeader *JWTLocationCookie) Validate() error {
	if cookieHeader.Name == "" {
		return fmt.Errorf("JWT location cookie name must be specified")
	}
	return nil
}

func (queryHeader *JWTLocationQueryParam) Validate() error {
	if queryHeader.Name == "" {
		return fmt.Errorf("JWT location query param name must be specified")
	}
	return nil
}

func (location *JWTLocation) Validate() error {
	hasHeader := location.Header != nil
	hasQueryParam := location.QueryParam != nil
	hasCookie := location.Cookie != nil

	hasAllThree := hasHeader && hasQueryParam && hasCookie

	if hasAllThree || !(hasCookie || hasQueryParam || hasHeader) {
		return fmt.Errorf("must set exactly one of: JWT location header, query param or cookie")
	}

	if hasHeader {
		if err := location.Header.Validate(); err != nil {
			return err
		}
		return nil
	}

	if hasCookie {
		if err := location.Cookie.Validate(); err != nil {
			return err
		}
		return nil
	}

	if err := location.QueryParam.Validate(); err != nil {
		return err
	}
	return nil
}

func validateLocations(locations []*JWTLocation) error {
	for _, location := range locations {
		if err := location.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (forwardingConfig *JWTForwardingConfig) Validate() error {
	if forwardingConfig.HeaderName == "" {
		return fmt.Errorf("header name required for forwarding config")
	}

	return nil
}

func (e *JWTProviderConfigEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}

	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	if e.JSONWebKeySet == nil {
		return fmt.Errorf("JSONWebKeySet is required")
	}

	if err := e.JSONWebKeySet.Validate(); err != nil {
		return err
	}

	if e.Locations != nil {
		validateLocations(e.Locations)
	}

	if e.Forwarding != nil {
		e.Forwarding.Validate()
	}

	return nil
}

func (e *JWTProviderConfigEntry) Normalize() error {
	// TODO - RONALD: FIGURE OUT WHAT TO NORMALIZE
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}
	return nil
}
