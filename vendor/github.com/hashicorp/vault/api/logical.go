package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/vault/sdk/helper/jsonutil"
)

const (
	wrappedResponseLocation = "cubbyhole/response"
)

var (
	// The default TTL that will be used with `sys/wrapping/wrap`, can be
	// changed
	DefaultWrappingTTL = "5m"

	// The default function used if no other function is set. It honors the env
	// var to set the wrap TTL. The default wrap TTL will apply when when writing
	// to `sys/wrapping/wrap` when the env var is not set.
	DefaultWrappingLookupFunc = func(operation, path string) string {
		if os.Getenv(EnvVaultWrapTTL) != "" {
			return os.Getenv(EnvVaultWrapTTL)
		}

		if (operation == "PUT" || operation == "POST") && path == "sys/wrapping/wrap" {
			return DefaultWrappingTTL
		}

		return ""
	}
)

// Logical is used to perform logical backend operations on Vault.
type Logical struct {
	c *Client
}

// Logical is used to return the client for logical-backend API calls.
func (c *Client) Logical() *Logical {
	return &Logical{c: c}
}

func (c *Logical) Read(path string) (*Secret, error) {
	return c.ReadWithData(path, nil)
}

func (c *Logical) ReadWithData(path string, data map[string][]string) (*Secret, error) {
	r := c.c.NewRequest("GET", "/v1/"+path)

	var values url.Values
	for k, v := range data {
		if values == nil {
			values = make(url.Values)
		}
		for _, val := range v {
			values.Add(k, val)
		}
	}

	if values != nil {
		r.Params = values
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	resp, err := c.c.RawRequestWithContext(ctx, r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == 404 {
		secret, parseErr := ParseSecret(resp.Body)
		switch parseErr {
		case nil:
		case io.EOF:
			return nil, nil
		default:
			return nil, err
		}
		if secret != nil && (len(secret.Warnings) > 0 || len(secret.Data) > 0) {
			return secret, nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return ParseSecret(resp.Body)
}

func (c *Logical) List(path string) (*Secret, error) {
	r := c.c.NewRequest("LIST", "/v1/"+path)
	// Set this for broader compatibility, but we use LIST above to be able to
	// handle the wrapping lookup function
	r.Method = "GET"
	r.Params.Set("list", "true")

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	resp, err := c.c.RawRequestWithContext(ctx, r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == 404 {
		secret, parseErr := ParseSecret(resp.Body)
		switch parseErr {
		case nil:
		case io.EOF:
			return nil, nil
		default:
			return nil, err
		}
		if secret != nil && (len(secret.Warnings) > 0 || len(secret.Data) > 0) {
			return secret, nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return ParseSecret(resp.Body)
}

func (c *Logical) Write(path string, data map[string]interface{}) (*Secret, error) {
	r := c.c.NewRequest("PUT", "/v1/"+path)
	if err := r.SetJSONBody(data); err != nil {
		return nil, err
	}

	return c.write(path, r)
}

func (c *Logical) WriteBytes(path string, data []byte) (*Secret, error) {
	r := c.c.NewRequest("PUT", "/v1/"+path)
	r.BodyBytes = data

	return c.write(path, r)
}

func (c *Logical) write(path string, request *Request) (*Secret, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	resp, err := c.c.RawRequestWithContext(ctx, request)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == 404 {
		secret, parseErr := ParseSecret(resp.Body)
		switch parseErr {
		case nil:
		case io.EOF:
			return nil, nil
		default:
			return nil, err
		}
		if secret != nil && (len(secret.Warnings) > 0 || len(secret.Data) > 0) {
			return secret, err
		}
	}
	if err != nil {
		return nil, err
	}

	return ParseSecret(resp.Body)
}

func (c *Logical) Delete(path string) (*Secret, error) {
	return c.DeleteWithData(path, nil)
}

func (c *Logical) DeleteWithData(path string, data map[string][]string) (*Secret, error) {
	r := c.c.NewRequest("DELETE", "/v1/"+path)

	var values url.Values
	for k, v := range data {
		if values == nil {
			values = make(url.Values)
		}
		for _, val := range v {
			values.Add(k, val)
		}
	}

	if values != nil {
		r.Params = values
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	resp, err := c.c.RawRequestWithContext(ctx, r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == 404 {
		secret, parseErr := ParseSecret(resp.Body)
		switch parseErr {
		case nil:
		case io.EOF:
			return nil, nil
		default:
			return nil, err
		}
		if secret != nil && (len(secret.Warnings) > 0 || len(secret.Data) > 0) {
			return secret, err
		}
	}
	if err != nil {
		return nil, err
	}

	return ParseSecret(resp.Body)
}

func (c *Logical) Unwrap(wrappingToken string) (*Secret, error) {
	var data map[string]interface{}
	if wrappingToken != "" {
		if c.c.Token() == "" {
			c.c.SetToken(wrappingToken)
		} else if wrappingToken != c.c.Token() {
			data = map[string]interface{}{
				"token": wrappingToken,
			}
		}
	}

	r := c.c.NewRequest("PUT", "/v1/sys/wrapping/unwrap")
	if err := r.SetJSONBody(data); err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	resp, err := c.c.RawRequestWithContext(ctx, r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp == nil || resp.StatusCode != 404 {
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, nil
		}
		return ParseSecret(resp.Body)
	}

	// In the 404 case this may actually be a wrapped 404 error
	secret, parseErr := ParseSecret(resp.Body)
	switch parseErr {
	case nil:
	case io.EOF:
		return nil, nil
	default:
		return nil, err
	}
	if secret != nil && (len(secret.Warnings) > 0 || len(secret.Data) > 0) {
		return secret, nil
	}

	// Otherwise this might be an old-style wrapping token so attempt the old
	// method
	if wrappingToken != "" {
		origToken := c.c.Token()
		defer c.c.SetToken(origToken)
		c.c.SetToken(wrappingToken)
	}

	secret, err = c.Read(wrappedResponseLocation)
	if err != nil {
		return nil, errwrap.Wrapf(fmt.Sprintf("error reading %q: {{err}}", wrappedResponseLocation), err)
	}
	if secret == nil {
		return nil, fmt.Errorf("no value found at %q", wrappedResponseLocation)
	}
	if secret.Data == nil {
		return nil, fmt.Errorf("\"data\" not found in wrapping response")
	}
	if _, ok := secret.Data["response"]; !ok {
		return nil, fmt.Errorf("\"response\" not found in wrapping response \"data\" map")
	}

	wrappedSecret := new(Secret)
	buf := bytes.NewBufferString(secret.Data["response"].(string))
	if err := jsonutil.DecodeJSONFromReader(buf, wrappedSecret); err != nil {
		return nil, errwrap.Wrapf("error unmarshalling wrapped secret: {{err}}", err)
	}

	return wrappedSecret, nil
}
