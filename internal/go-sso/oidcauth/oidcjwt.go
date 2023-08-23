package oidcauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/internal/go-sso/oidcauth/internal/strutil"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/pointerstructure"
	"golang.org/x/oauth2"
)

func contextWithHttpClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, client)
}

func createHTTPClient(caCert string) (*http.Client, error) {
	tr := cleanhttp.DefaultPooledTransport()

	if caCert != "" {
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM([]byte(caCert)); !ok {
			return nil, errors.New("could not parse CA PEM value successfully")
		}

		tr.TLSClientConfig = &tls.Config{
			RootCAs: certPool,
		}
	}

	return &http.Client{
		Transport: tr,
	}, nil
}

// extractClaims extracts all configured claims from the received claims.
func (a *Authenticator) extractClaims(allClaims map[string]interface{}) (*Claims, error) {
	metadata, err := extractStringMetadata(a.logger, allClaims, a.config.ClaimMappings)
	if err != nil {
		return nil, err
	}

	listMetadata, err := extractListMetadata(a.logger, allClaims, a.config.ListClaimMappings)
	if err != nil {
		return nil, err
	}

	return &Claims{
		Values: metadata,
		Lists:  listMetadata,
	}, nil
}

// extractStringMetadata builds a metadata map of string values from a set of
// claims and claims mappings.  The referenced claims must be strings and the
// claims mappings must be of the structure:
//
//	{
//	    "/some/claim/pointer": "metadata_key1",
//	    "another_claim": "metadata_key2",
//	     ...
//	}
func extractStringMetadata(logger hclog.Logger, allClaims map[string]interface{}, claimMappings map[string]string) (map[string]string, error) {
	metadata := make(map[string]string)
	for source, target := range claimMappings {
		rawValue := getClaim(logger, allClaims, source)
		if rawValue == nil {
			continue
		}

		strValue, ok := stringifyMetadataValue(rawValue)
		if !ok {
			return nil, fmt.Errorf("error converting claim '%s' to string from unknown type %T", source, rawValue)
		}

		metadata[target] = strValue
	}
	return metadata, nil
}

// extractListMetadata builds a metadata map of string list values from a set
// of claims and claims mappings.  The referenced claims must be strings and
// the claims mappings must be of the structure:
//
//	{
//	    "/some/claim/pointer": "metadata_key1",
//	    "another_claim": "metadata_key2",
//	     ...
//	}
func extractListMetadata(logger hclog.Logger, allClaims map[string]interface{}, listClaimMappings map[string]string) (map[string][]string, error) {
	out := make(map[string][]string)
	for source, target := range listClaimMappings {
		if rawValue := getClaim(logger, allClaims, source); rawValue != nil {
			rawList, ok := normalizeList(rawValue)
			if !ok {
				return nil, fmt.Errorf("%q list claim could not be converted to string list", source)
			}

			list := make([]string, 0, len(rawList))
			for _, raw := range rawList {
				value, ok := stringifyMetadataValue(raw)
				if !ok {
					return nil, fmt.Errorf("value %v in %q list claim could not be parsed as string", raw, source)
				}

				if value == "" {
					continue
				}
				list = append(list, value)
			}

			out[target] = list
		}
	}
	return out, nil
}

// getClaim returns a claim value from allClaims given a provided claim string.
// If this string is a valid JSONPointer, it will be interpreted as such to
// locate the claim. Otherwise, the claim string will be used directly.
//
// There is no fixup done to the returned data type here. That happens a layer
// up in the caller.
func getClaim(logger hclog.Logger, allClaims map[string]interface{}, claim string) interface{} {
	if !strings.HasPrefix(claim, "/") {
		return allClaims[claim]
	}

	val, err := pointerstructure.Get(allClaims, claim)
	if err != nil {
		if logger != nil {
			logger.Warn("unable to locate claim", "claim", claim, "error", err)
		}
		return nil
	}

	return val
}

// normalizeList takes an item or a slice and returns a slice. This is useful
// when providers are expected to return a list (typically of strings) but
// reduce it to a non-slice type when the list count is 1.
//
// There is no fixup done to elements of the returned slice here. That happens
// a layer up in the caller.
func normalizeList(raw interface{}) ([]interface{}, bool) {
	switch v := raw.(type) {
	case []interface{}:
		return v, true
	case string, // note: this list should be the same as stringifyMetadataValue
		bool,
		json.Number,
		float64,
		float32,
		int8,
		int16,
		int32,
		int64,
		int,
		uint8,
		uint16,
		uint32,
		uint64,
		uint:
		return []interface{}{v}, true
	default:
		return nil, false
	}

}

// stringifyMetadataValue will try to convert the provided raw value into a
// faithful string representation of that value per these rules:
//
// - strings      => unchanged
// - bool         => "true" / "false"
// - json.Number  => String()
// - float32/64   => truncated to int64 and then formatted as an ascii string
// - intXX/uintXX => casted to int64 and then formatted as an ascii string
//
// If successful the string value and true are returned. otherwise an empty
// string and false are returned.
func stringifyMetadataValue(rawValue interface{}) (string, bool) {
	switch v := rawValue.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case json.Number:
		return v.String(), true
	case float64:
		// The claims unmarshalled by go-oidc don't use UseNumber, so
		// they'll come in as float64 instead of an integer or json.Number.
		return strconv.FormatInt(int64(v), 10), true

		// The numerical type cases following here are only here for the sake
		// of numerical type completion. Everything is truncated to an integer
		// before being stringified.
	case float32:
		return strconv.FormatInt(int64(v), 10), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case int:
		return strconv.FormatInt(int64(v), 10), true
	case uint8:
		return strconv.FormatInt(int64(v), 10), true
	case uint16:
		return strconv.FormatInt(int64(v), 10), true
	case uint32:
		return strconv.FormatInt(int64(v), 10), true
	case uint64:
		return strconv.FormatInt(int64(v), 10), true
	case uint:
		return strconv.FormatInt(int64(v), 10), true
	default:
		return "", false
	}
}

// validateAudience checks whether any of the audiences in audClaim match those
// in boundAudiences. If strict is true and there are no bound audiences, then
// the presence of any audience in the received claim is considered an error.
func validateAudience(boundAudiences, audClaim []string, strict bool) error {
	if strict && len(boundAudiences) == 0 && len(audClaim) > 0 {
		return errors.New("audience claim found in JWT but no audiences are bound")
	}

	if len(boundAudiences) > 0 {
		for _, v := range boundAudiences {
			if strutil.StrListContains(audClaim, v) {
				return nil
			}
		}
		return errors.New("aud claim does not match any bound audience")
	}

	return nil
}
