// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package netutil

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAgentConfig(t *testing.T) {
	// Test cases
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		expectError   bool
		expectedError string
		expectedData  map[string]interface{}
	}{
		{
			name: "successful config retrieval",
			handler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v1/agent/self", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"Config": map[string]interface{}{
						"NodeName":    "test-node",
						"Datacenter":  "dc1",
						"Server":      true,
						"DataDir":     "/tmp/consul",
						"LogLevel":    "info",
						"NodeID":      "11111111-2222-3333-4444-555555555555",
						"RetryJoin":   []string{"127.0.0.1:8301"},
						"BindAddr":    "127.0.0.1",
						"ClientAddr":  "127.0.0.1",
						"HTTPPort":    8500,
						"DNS":         map[string]interface{}{"Port": 8600},
						"ServerName":  "test-server",
						"PidFile":     "/tmp/consul/consul.pid",
						"Performance": map[string]interface{}{"RaftMultiplier": 1},
					},
				})
			},
			expectError: false,
			expectedData: map[string]interface{}{
				"Config": map[string]interface{}{
					"NodeName": "test-node",
				},
			},
		},
		{
			name: "connection error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("connection refused"))
			},
			expectError:   true,
			expectedError: "Unexpected response code: 500 (connection refused)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test HTTP server
			ts := httptest.NewServer(tc.handler)
			defer ts.Close()

			// Set the CONSUL_HTTP_ADDR environment variable to point to our test server
			oldEnv := os.Getenv("CONSUL_HTTP_ADDR")
			os.Setenv("CONSUL_HTTP_ADDR", ts.URL)
			defer os.Setenv("CONSUL_HTTP_ADDR", oldEnv)

			// Call the function under test
			result, err := GetAgentConfig()

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedError != "" {
					require.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Verify the expected data is in the result
				if tc.expectedData != nil {
					for key, expectedVal := range tc.expectedData {
						actualVal, ok := result[key]
						require.True(t, ok, "expected key %s not found in result", key)

						// For nested maps, check each key-value pair
						if expectedMap, ok := expectedVal.(map[string]interface{}); ok {
							actualVal := result[key]
							require.NotNil(t, actualVal, "expected %s to be a map, got nil", key)

							for k, v := range expectedMap {
								actualNestedVal, ok := actualVal[k]
								require.True(t, ok, "expected key %s.%s not found in result", key, k)
								require.Equal(t, v, actualNestedVal, "mismatch for %s.%s", key, k)
							}
						} else {
							require.Equal(t, expectedVal, actualVal, "mismatch for %s", key)
						}
					}
				}
			}
		})
	}
}

func TestIsDualStack(t *testing.T) {
	// Save the original GetAgentBindAddrFunc and restore after test
	originalGetAgentBindAddr := GetAgentBindAddrFunc
	defer func() { GetAgentBindAddrFunc = originalGetAgentBindAddr }()

	tests := []struct {
		name            string
		mockBindIP      string
		setupMock       func()
		expectDualStack bool
		expectError     bool
		expectedError   string
	}{
		{
			name:            "IPv4 address",
			mockBindIP:      "192.168.1.1",
			expectDualStack: false,
			expectError:     false,
		},
		{
			name:            "IPv6 address",
			mockBindIP:      "2001:db8::1",
			expectDualStack: true,
			expectError:     false,
		},
		{
			name:            "IPv4-mapped IPv6 address",
			mockBindIP:      "::ffff:192.168.1.1",
			expectDualStack: false,
			expectError:     false,
		},
		{
			name:            "empty bind address",
			mockBindIP:      "",
			expectDualStack: false,
			expectError:     false,
		},
		{
			name: "invalid IP address",
			setupMock: func() {
				GetAgentBindAddrFunc = func() (net.IP, error) {
					return nil, fmt.Errorf("unable to parse bind address")
				}
			},
			expectError:   true,
			expectedError: "unable to parse bind address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMock != nil {
				tc.setupMock()
			} else {
				// Set up a default mock for GetAgentBindAddr
				GetAgentBindAddrFunc = func() (net.IP, error) {
					if tc.mockBindIP == "" {
						return nil, nil
					}
					ip := net.ParseIP(tc.mockBindIP)
					if ip == nil {
						return nil, fmt.Errorf("unable to parse bind address")
					}
					return ip, nil
				}
			}

			// Call the function under test
			isDualStack, err := IsDualStack()

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedError != "" {
					require.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectDualStack, isDualStack)
			}
		})
	}
}

func TestGetAgentBindAddr(t *testing.T) {
	// Save original GetAgentConfigFunc and restore after test
	originalGetAgentConfig := GetAgentConfigFunc
	defer func() { GetAgentConfigFunc = originalGetAgentConfig }()

	tests := []struct {
		name          string
		mockConfig    map[string]map[string]interface{}
		expectError   bool
		expectedError string
		expectedIP    string
		setupMock     func()
	}{
		{
			name: "successful bind address retrieval",
			mockConfig: map[string]map[string]interface{}{
				"Config": {
					"BindAddr": "192.168.1.100",
				},
			},
			expectError: false,
			expectedIP:  "192.168.1.100",
		},
		{
			name: "empty bind address",
			mockConfig: map[string]map[string]interface{}{
				"Config": {
					"BindAddr": "",
				},
			},
			expectError: false,
			expectedIP:  "",
		},
		{
			name: "missing bind address",
			mockConfig: map[string]map[string]interface{}{
				"Config": {
					"OtherField": "value",
				},
			},
			expectError: false,
			expectedIP:  "",
		},
		{
			name: "invalid bind address format",
			mockConfig: map[string]map[string]interface{}{
				"Config": {
					"BindAddr": "not.an.ip.address",
				},
			},
			expectError:   true,
			expectedError: "ParseAddr(\"not.an.ip.address\"): unexpected character (at \"not.an.ip.address\")",
		},
		{
			name:          "GetAgentConfig returns error",
			mockConfig:    nil,
			expectError:   true,
			expectedError: "failed to get agent config",
			setupMock: func() {
				GetAgentConfigFunc = func() (map[string]map[string]interface{}, error) {
					return nil, fmt.Errorf("failed to get agent config")
				}
				GetAgentBindAddrFunc = func() (net.IP, error) {
					return nil, fmt.Errorf("failed to get bind address")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call setupMock if it exists
			if tc.setupMock != nil {
				tc.setupMock()
			} else {
				// Default mock setup if no setupMock provided
				GetAgentConfigFunc = func() (map[string]map[string]interface{}, error) {
					if tc.mockConfig == nil {
						return nil, fmt.Errorf("mock error")
					}
					return tc.mockConfig, nil
				}
			}

			// Call the function under test
			ip, err := GetAgentBindAddr()

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedError != "" {
					require.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				require.NoError(t, err)
				if tc.expectedIP == "" {
					require.Nil(t, ip)
				} else {
					require.NotNil(t, ip)
					require.Equal(t, tc.expectedIP, ip.String())
				}
			}
		})
	}
}
