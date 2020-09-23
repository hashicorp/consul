package uiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestUIServer(t *testing.T) {
	cases := []struct {
		name            string
		cfg             *config.RuntimeConfig
		path            string
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
			wantEnv: map[string]interface{}{
				"CONSUL_ACLS_ENABLED": false,
			},
		},
		{
			// TODO: is this really what we want? It's what we've always done but
			// seems a bit odd to not do an actual 301 but instead serve the
			// index.html from every path... It also breaks the UI probably.
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
				withMetricsProviderOptions(`{"bar":1}`),
			),
			path:       "/",
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<!-- CONSUL_VERSION:",
			},
			wantEnv: map[string]interface{}{
				"CONSUL_ACLS_ENABLED": false,
			},
			wantUICfgJSON: `{
				"metrics_provider":         "foo",
				"metrics_provider_options": {
					"bar":1
				},
				"metrics_proxy_enabled": false,
				"dashboard_url_templates": null
			}`,
		},
		{
			name:         "acls enabled",
			cfg:          basicUIEnabledConfig(withACLs()),
			path:         "/",
			wantStatus:   http.StatusOK,
			wantContains: []string{"<!-- CONSUL_VERSION:"},
			wantEnv: map[string]interface{}{
				"CONSUL_ACLS_ENABLED": true,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(tc.cfg, testutil.Logger(t))

			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			for _, want := range tc.wantContains {
				require.Contains(t, rec.Body.String(), want)
			}
			for _, wantNot := range tc.wantNotContains {
				require.NotContains(t, rec.Body.String(), wantNot)
			}
			env := extractEnv(t, rec.Body.String())
			for k, v := range tc.wantEnv {
				require.Equal(t, v, env[k])
			}
			if tc.wantUICfgJSON != "" {
				require.JSONEq(t, tc.wantUICfgJSON, extractUIConfig(t, rec.Body.String()))
			}
		})
	}
}

func extractMetaJSON(t *testing.T, name, content string) string {
	t.Helper()

	// Find and extract the env meta tag. Why yes I _am_ using regexp to parse
	// HTML thanks for asking. In this case it's HTML with a very limited format
	// so I don't feel too bad but maybe I should.
	// https://stackoverflow.com/questions/1732348/regex-match-open-tags-except-xhtml-self-contained-tags/1732454#1732454
	re := regexp.MustCompile(`<meta name="` + name + `+" content="([^"]*)"`)

	matches := re.FindStringSubmatch(content)
	require.Len(t, matches, 2, "didn't find the %s meta tag", name)

	// Unescape the JSON
	jsonStr, err := url.PathUnescape(matches[1])
	require.NoError(t, err)

	return jsonStr
}

func extractEnv(t *testing.T, content string) map[string]interface{} {
	t.Helper()

	js := extractMetaJSON(t, "consul-ui/config/environment", content)

	var env map[string]interface{}

	err := json.Unmarshal([]byte(js), &env)
	require.NoError(t, err)

	return env
}

func extractUIConfig(t *testing.T, content string) string {
	t.Helper()
	return extractMetaJSON(t, "consul-ui/ui_config", content)
}

type cfgFunc func(cfg *config.RuntimeConfig)

func basicUIEnabledConfig(opts ...cfgFunc) *config.RuntimeConfig {
	cfg := &config.RuntimeConfig{
		UIConfig: config.UIConfig{
			Enabled: true,
		},
	}
	for _, f := range opts {
		f(cfg)
	}
	return cfg
}

func withACLs() cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.ACLDatacenter = "dc1"
		cfg.ACLDefaultPolicy = "deny"
		cfg.ACLsEnabled = true
	}
}

func withMetricsProvider(name string) cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.MetricsProvider = name
	}
}

func withMetricsProviderOptions(jsonStr string) cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.MetricsProviderOptionsJSON = jsonStr
	}
}

// TestMultipleIndexRequests validates that the buffered file mechanism works
// beyond the first request. The initial implementation did not as it shared an
// bytes.Reader between callers.
func TestMultipleIndexRequests(t *testing.T) {
	h := NewHandler(basicUIEnabledConfig(), testutil.Logger(t))

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
	h := NewHandler(basicUIEnabledConfig(), testutil.Logger(t))

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
