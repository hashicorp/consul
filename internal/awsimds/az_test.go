// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package awsimds

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLookupAvailabilityZone_IMDSv2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, tokenTTL, r.Header.Get(tokenTTLHeader))
			_, err := w.Write([]byte("token-123"))
			require.NoError(t, err)
		case "/latest/meta-data/placement/availability-zone":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "token-123", r.Header.Get(tokenHeader))
			_, err := w.Write([]byte("eu-west-1a\n"))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	restore := setTestEndpoints(server.URL)
	defer restore()

	zone, err := LookupAvailabilityZone(context.Background(), server.Client())
	require.NoError(t, err)
	require.Equal(t, "eu-west-1a", zone)
}

func TestLookupAvailabilityZone_IMDSv1Fallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			http.Error(w, "forbidden", http.StatusForbidden)
		case "/latest/meta-data/placement/availability-zone":
			require.Equal(t, "", r.Header.Get(tokenHeader))
			_, err := w.Write([]byte("eu-west-1b"))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	restore := setTestEndpoints(server.URL)
	defer restore()

	zone, err := LookupAvailabilityZone(context.Background(), server.Client())
	require.NoError(t, err)
	require.Equal(t, "eu-west-1b", zone)
}

func TestLookupAvailabilityZone_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	restore := setTestEndpoints(server.URL)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := LookupAvailabilityZone(ctx, server.Client())
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestLookupAvailabilityZone_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			_, err := w.Write([]byte("token-123"))
			require.NoError(t, err)
		case "/latest/meta-data/placement/availability-zone":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	restore := setTestEndpoints(server.URL)
	defer restore()

	_, err := LookupAvailabilityZone(context.Background(), server.Client())
	require.Error(t, err)
	require.ErrorContains(t, err, "empty response body")
}

func setTestEndpoints(baseURL string) func() {
	prevTokenEndpoint := tokenEndpoint
	prevAvailabilityZoneEndpoint := availabilityZoneEndpoint

	tokenEndpoint = baseURL + "/latest/api/token"
	availabilityZoneEndpoint = baseURL + "/latest/meta-data/placement/availability-zone"

	return func() {
		tokenEndpoint = prevTokenEndpoint
		availabilityZoneEndpoint = prevAvailabilityZoneEndpoint
	}
}
