package uiserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
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
			wantNotContains: []string{
				"__RUNTIME_BOOL_",
				"__RUNTIME_STRING_",
			},
			wantEnv: map[string]interface{}{
				"CONSUL_ACLS_ENABLED":     false,
				"CONSUL_DATACENTER_LOCAL": "dc1",
			},
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
			wantNotContains: []string{
				// This is a quick check to be sure that we actually URL encoded the
				// JSON ui settings too. The assertions below could pass just fine even
				// if we got that wrong because the decode would be a no-op if it wasn't
				// URL encoded. But this just ensures that we don't see the raw values
				// in the output because the quotes should be encoded.
				`"a-very-unlikely-string"`,
			},
			wantEnv: map[string]interface{}{
				"CONSUL_ACLS_ENABLED": false,
			},
			wantUICfgJSON: `{
				"metrics_provider":         "foo",
				"metrics_provider_options": {
					"a-very-unlikely-string":1
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
		{
			name: "external transformation",
			cfg: basicUIEnabledConfig(
				withMetricsProvider("foo"),
			),
			path: "/",
			tx: func(cfg *config.RuntimeConfig, data map[string]interface{}) error {
				data["SSOEnabled"] = true
				o := data["UIConfig"].(map[string]interface{})
				o["metrics_provider"] = "bar"
				return nil
			},
			wantStatus: http.StatusOK,
			wantContains: []string{
				"<!-- CONSUL_VERSION:",
			},
			wantEnv: map[string]interface{}{
				"CONSUL_SSO_ENABLED": true,
			},
			wantUICfgJSON: `{
				"metrics_provider": "bar",
				"metrics_proxy_enabled": false,
				"dashboard_url_templates": null
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
		{
			name: "injecting content path",
			// ember will always pre/append slashes if they don't exist so even if
			// somebody configures them without our template file will have them
			cfg: basicUIEnabledConfig(
				withContentPath("/consul/"),
			),
			path:       "/",
			wantStatus: http.StatusOK,
			wantNotContains: []string{
				"__RUNTIME_BOOL_",
				"__RUNTIME_STRING_",
			},
			wantEnv: map[string]interface{}{
				"rootURL": "/consul/",
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
			Enabled:     true,
			ContentPath: "/ui/",
		},
		Datacenter: "dc1",
	}
	for _, f := range opts {
		f(cfg)
	}
	return cfg
}

func withContentPath(path string) cfgFunc {
	return func(cfg *config.RuntimeConfig) {
		cfg.UIConfig.ContentPath = path
	}
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
	require.NoError(t, ioutil.WriteFile(path, []byte("test"), 0644))

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
			wantCompiled, err := ioutil.ReadFile("testdata/compiled-metrics-providers-golden.js")
			require.NoError(t, err)
			require.Equal(t, rec.Body.String(), string(wantCompiled))
		})
	}

}
