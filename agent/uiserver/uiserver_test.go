// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package uiserver

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestUIServerIndex(t *testing.T) {
	cases := []struct {
		name            string
		cfg             *config.RuntimeConfig
		path            string
		tx              UIDataTransform
		wantStatus      int
		wantContains    []string
		wantNotContains []string
		wantEnv         map[string]interface{}
		wantUICfgJSON   string
	}{
		{
			name:         "basic UI serving",
			cfg:          basicUIEnabledConfig(),
			path:         "/", // Note /index.html redirects to /
			wantStatus:   http.StatusOK,
			wantContains: []string{"<!-- CONSUL_VERSION:"},
			wantUICfgJSON: `{
				"ACLsEnabled": false,
				"HCPEnabled": false,
				"LocalDatacenter": "dc1",
				"PrimaryDatacenter": "dc1",
				"ContentPath": "/ui/",
				"PeeringEnabled": true,
				"UIConfig": {
					"hcp_enabled": false,
					"metrics_provider": "",
					"metrics_proxy_enabled": false,
					"dashboard_url_templates": null
				}
			}`,
		},
		{
			// We do this redirect just for UI dir since the app is a single page app
			// and any URL under the path should just load the index and let Ember do
			// it's thing unless it's a specific asset URL in the filesystem.
			name:         "unknown paths to serve index",
			cfg:          basicUIEnabledConfig(),
			path:         "/foo-bar-bazz-qux",
			wantStatus:   http.StatusOK,
			wantContains: []string{"<!-- CONSUL_VERSION:"},
		},
		{
			name: "injecting metrics vars",
			cfg: basicUIEnabledConfig(
				withMetricsProvider("foo"),
				withMetricsProviderOptions(`{"a-very-unlikely-string":1}`),
			),
			path:       "/",
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<!-- CONSUL_VERSION:",
			},
			wantUICfgJSON: `{
				"ACLsEnabled": false,
				"HCPEnabled": false,
				"LocalDatacenter": "dc1",
				"PrimaryDatacenter": "dc1",
				"ContentPath": "/ui/",
				"PeeringEnabled": true,
				"UIConfig": {
					"hcp_enabled": false,
					"metrics_provider": "foo",
					"metrics_provider_options": {
						"a-very-unlikely-string":1
					},
					"metrics_proxy_enabled": false,
					"dashboard_url_templates": null
				}
			}`,
		},
		{
			name:         "acls enabled",
			cfg:          basicUIEnabledConfig(withACLs()),
			path:         "/",
			wantStatus:   http.StatusOK,
			wantContains: []string{"<!-- CONSUL_VERSION:"},
			wantUICfgJSON: `{
				"ACLsEnabled": true,
				"HCPEnabled": false,
				"LocalDatacenter": "dc1",
				"PrimaryDatacenter": "dc1",
				"ContentPath": "/ui/",
				"PeeringEnabled": true,
				"UIConfig": {
					"hcp_enabled": false,
					"metrics_provider": "",
					"metrics_proxy_enabled": false,
					"dashboard_url_templates": null
				}
			}`,
		},
		{
			name:         "hcp enabled",
			cfg:          basicUIEnabledConfig(withHCPEnabled()),
			path:         "/",
			wantStatus:   http.StatusOK,
			wantContains: []string{"<!-- CONSUL_VERSION:"},
			wantUICfgJSON: `{
				"ACLsEnabled": false,
				"HCPEnabled": true,
				"LocalDatacenter": "dc1",
				"PrimaryDatacenter": "dc1",
				"ContentPath": "/ui/",
				"PeeringEnabled": true,
				"UIConfig": {
					"hcp_enabled": true,
					"metrics_provider": "",
					"metrics_proxy_enabled": false,
					"dashboard_url_templates": null
				}
			}`,
		},
		{
			name: "peering disabled",
			cfg: basicUIEnabledConfig(
				withPeeringDisabled(),
			),
			path:         "/",
			wantStatus:   http.StatusOK,
			wantContains: []string{"<!-- CONSUL_VERSION:"},
			wantUICfgJSON: `{
				"ACLsEnabled": false,
				"HCPEnabled": false,
				"LocalDatacenter": "dc1",
				"PrimaryDatacenter": "dc1",
				"ContentPath": "/ui/",
				"PeeringEnabled": false,
				"UIConfig": {
					"hcp_enabled": false,
					"metrics_provider": "",
					"metrics_proxy_enabled": false,
					"dashboard_url_templates": null
				}
			}`,
		},
		{
			name: "external transformation",
			cfg: basicUIEnabledConfig(
				withMetricsProvider("foo"),
			),
			path: "/",
			tx: func(data map[string]interface{}) error {
				data["SSOEnabled"] = true
				o := data["UIConfig"].(map[string]interface{})
				o["metrics_provider"] = "bar"
				return nil
			},
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<!-- CONSUL_VERSION:",
			},
			wantUICfgJSON: `{
				"ACLsEnabled": false,
				"HCPEnabled": false,
				"SSOEnabled": true,
				"LocalDatacenter": "dc1",
				"PrimaryDatacenter": "dc1",
				"ContentPath": "/ui/",
				"PeeringEnabled": true,
				"UIConfig": {
					"hcp_enabled": false,
					"metrics_provider": "bar",
					"metrics_proxy_enabled": false,
					"dashboard_url_templates": null
				}
			}`,
		},
		{
			name: "serving metrics provider js",
			cfg: basicUIEnabledConfig(
				withMetricsProvider("foo"),
				withMetricsProviderFiles("testdata/foo.js", "testdata/bar.js"),
			),
			path:       "/",
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<!-- CONSUL_VERSION:",
				`<script src="/ui/assets/compiled-metrics-providers.js">`,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(tc.cfg, testutil.Logger(t), tc.tx)

			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			for _, want := range tc.wantContains {
				require.Contains(t, rec.Body.String(), want)
			}
			if tc.wantUICfgJSON != "" {
				require.JSONEq(t, tc.wantUICfgJSON, extractUIConfig(t, rec.Body.String()))
			}
		})
	}
}

func extractApplicationJSON(t *testing.T, attrName, content string) string {
	t.Helper()

	var scriptContent *html.Node
	var find func(node *html.Node)

	// Recurse down the tree and pick out <script attrName=ourAttrName>
	find = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "script" {
			for i := 0; i < len(node.Attr); i++ {
				attr := node.Attr[i]
				if attr.Key == attrName {
					// find the script and save off the content, which in this case is
					// the JSON we are looking for, once we have it finish up
					scriptContent = node.FirstChild
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}

	doc, err := html.Parse(strings.NewReader(content))
	require.NoError(t, err)

	find(doc)

	var buf bytes.Buffer
	w := io.Writer(&buf)

	renderErr := html.Render(w, scriptContent)
	require.NoError(t, renderErr)

	jsonStr := html.UnescapeString(buf.String())
	return jsonStr
}

func extractUIConfig(t *testing.T, content string) string {
	t.Helper()
	return extractApplicationJSON(t, "data-consul-ui-config", content)
}

type cfgFunc func(cfg *config.RuntimeConfig)

func basicUIEnabledConfig(opts ...cfgFunc) *config.RuntimeConfig {
	cfg := &config.RuntimeConfig{
		UIConfig: config.UIConfig{
			Enabled:     true,
			ContentPath: "/ui/",
		},
		Datacenter:        "dc1",
		PrimaryDatacenter: "dc1",
		PeeringEnabled:    true,
	}
	for _, f := range opts {
		f(cfg)
	}
	return cfg
}

func withACLs() cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.PrimaryDatacenter = "dc1"
		cfg.ACLResolverSettings.ACLDefaultPolicy = "deny"
		cfg.ACLsEnabled = true
	}
}

func withMetricsProvider(name string) cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.MetricsProvider = name
	}
}

func withMetricsProviderFiles(names ...string) cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.MetricsProviderFiles = names
	}
}

func withMetricsProviderOptions(jsonStr string) cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.MetricsProviderOptionsJSON = jsonStr
	}
}

func withHCPEnabled() cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.HCPEnabled = true
	}
}

func withPeeringDisabled() cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.PeeringEnabled = false
	}
}

// TestMultipleIndexRequests validates that the buffered file mechanism works
// beyond the first request. The initial implementation did not as it shared an
// bytes.Reader between callers.
func TestMultipleIndexRequests(t *testing.T) {
	h := NewHandler(basicUIEnabledConfig(), testutil.Logger(t), nil)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "<!-- CONSUL_VERSION:",
			"request %d didn't return expected content", i+1)
	}
}

func TestReload(t *testing.T) {
	h := NewHandler(basicUIEnabledConfig(), testutil.Logger(t), nil)

	{
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "<!-- CONSUL_VERSION:")
		require.NotContains(t, rec.Body.String(), "exotic-metrics-provider-name")
	}

	// Reload the config with the changed metrics provider name
	newCfg := basicUIEnabledConfig(
		withMetricsProvider("exotic-metrics-provider-name"),
	)
	h.ReloadConfig(newCfg)

	// Now we should see the new provider name in the output of index
	{
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "<!-- CONSUL_VERSION:")
		require.Contains(t, rec.Body.String(), "exotic-metrics-provider-name")
	}
}

func TestCustomDir(t *testing.T) {
	uiDir := testutil.TempDir(t, "consul-uiserver")
	defer os.RemoveAll(uiDir)

	path := filepath.Join(uiDir, "test-file")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	cfg := basicUIEnabledConfig()
	cfg.UIConfig.Dir = uiDir
	h := NewHandler(cfg, testutil.Logger(t), nil)

	req := httptest.NewRequest("GET", "/test-file", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "test")
}

func TestCompiledJS(t *testing.T) {
	cfg := basicUIEnabledConfig(
		withMetricsProvider("foo"),
		withMetricsProviderFiles("testdata/foo.js", "testdata/bar.js"),
	)
	h := NewHandler(cfg, testutil.Logger(t), nil)

	paths := []string{
		"/" + compiledProviderJSPath,
		// We need to work even without the initial slash because the agent uses
		// http.StripPrefix with the entire ContentPath which includes a trailing
		// slash. This apparently works fine for the assetFS etc. so we need to
		// also tolerate it when the URL doesn't have a slash at the start of the
		// path.
		compiledProviderJSPath,
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			// NewRequest doesn't like paths with no leading slash but we need to test
			// a request with a URL that has that so just create with root path and
			// then manually modify the URL path so it emulates one that has been
			// doctored by http.StripPath.
			req := httptest.NewRequest("GET", "/", nil)
			req.URL.Path = path

			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, rec.Result().Header["Content-Type"][0], "application/javascript")
			wantCompiled, err := os.ReadFile("testdata/compiled-metrics-providers-golden.js")
			require.NoError(t, err)
			require.Equal(t, rec.Body.String(), string(wantCompiled))
		})
	}

}

func TestHandler_ServeHTTP_TransformIsEvaluatedOnEachRequest(t *testing.T) {
	cfg := basicUIEnabledConfig()

	value := "seeds"
	transform := func(data map[string]interface{}) error {
		data["apple"] = value
		return nil
	}
	h := NewHandler(cfg, hclog.New(nil), transform)

	t.Run("initial request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		expected := `{
		"ACLsEnabled": false,
		"HCPEnabled": false,
		"LocalDatacenter": "dc1",
		"PrimaryDatacenter": "dc1",
		"ContentPath": "/ui/",
		"PeeringEnabled": true,
		"UIConfig": {
			"hcp_enabled": false,
			"metrics_provider": "",
			"metrics_proxy_enabled": false,
			"dashboard_url_templates": null
		},
		"apple": "seeds"
	}`
		require.JSONEq(t, expected, extractUIConfig(t, rec.Body.String()))
	})

	t.Run("transform value has changed", func(t *testing.T) {

		value = "plant"
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		expected := `{
			"ACLsEnabled": false,
			"HCPEnabled": false,
			"LocalDatacenter": "dc1",
			"PrimaryDatacenter": "dc1",
			"ContentPath": "/ui/",
			"PeeringEnabled": true,
			"UIConfig": {
				"hcp_enabled": false,
				"metrics_provider": "",
				"metrics_proxy_enabled": false,
				"dashboard_url_templates": null
			},
			"apple": "plant"
		}`
		require.JSONEq(t, expected, extractUIConfig(t, rec.Body.String()))
	})
}
