package assert

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

const (
	defaultHTTPTimeout = 120 * time.Second
	defaultHTTPWait    = defaultWait
)

// HTTPServiceFailTcpConnection verifies that a TCP connection must not be allowed to ip:port
func HTTPServiceFailTcpConnection(t *testing.T, ip string, port int) {
	phrase := "hello"
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultHTTPTimeout, Wait: defaultHTTPWait}
	}

	client := http.DefaultClient
	urlreq := fmt.Sprintf("http://%s:%d", ip, port)

	retry.RunWith(failer(), t, func(r *retry.R) {
		t.Logf("making call to %s", urlreq)
		reader := strings.NewReader(phrase)
		_, err := client.Post(urlreq, "text/plain", reader)
		require.Error(r, err)

		urlErr, ok := err.(*url.Error)
		require.True(r, ok)
		require.Contains(r, urlErr.Unwrap().Error(), "EOF")
	})
}

// HTTPServiceEchoes verifies that a post to the given ip/port combination returns the data
// in the response body
func HTTPServiceEchoes(t *testing.T, ip string, port int) {
	phrase := "hello"
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultHTTPTimeout, Wait: defaultHTTPWait}
	}

	client := http.DefaultClient
	url := fmt.Sprintf("http://%s:%d", ip, port)

	retry.RunWith(failer(), t, func(r *retry.R) {
		t.Logf("making call to %s", url)
		reader := strings.NewReader(phrase)
		res, err := client.Post(url, "text/plain", reader)
		if err != nil {
			r.Fatal("could not make call to service ", url)
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			r.Fatal("could not read response body ", url)
		}

		if !strings.Contains(string(body), phrase) {
			r.Fatal("received an incorrect response ", body)
		}
	})
}

// CatalogServiceExists verifies the service name exists in the Consul catalog
func CatalogServiceExists(t *testing.T, c *api.Client, svc string) {
	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Catalog().Service(svc, "", nil)
		if err != nil {
			r.Fatal("error reading peering data")
		}
		if len(services) == 0 {
			r.Fatal("did not find catalog entry for ", svc)
		}
	})
}

// CatalogServiceHealthy verifies the service is healthy
func CatalogServiceHealthy(t *testing.T, c *api.Client, svc string) {
	retry.Run(t, func(r *retry.R) {
		services, _, err := c.Health().Service(svc, "", true, nil)
		if err != nil {
			r.Fatalf("client call: %w", err)
		}
		if len(services) == 0 {
			r.Fatalf("did not find catalog entry for %s", svc)
		}
	})
}
