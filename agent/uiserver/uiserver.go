package uiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"text/template"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
)

const (
	compiledProviderJSPath = "assets/compiled-metrics-providers.js"
)

// Handler is the http.Handler that serves the Consul UI. It may serve from the
// compiled-in AssetFS or from and external dir. It provides a few important
// transformations on the index.html file and includes a proxy for metrics
// backends.
type Handler struct {
	// state is a reloadableState struct accessed through an atomic value to make
	// it safe to reload at run time. Each call to ServeHTTP will see the latest
	// version of the state without internal locking needed.
	state     atomic.Value
	logger    hclog.Logger
	transform UIDataTransform
}

// reloadableState encapsulates all the state that might be modified during
// ReloadConfig.
type reloadableState struct {
	cfg *config.UIConfig
	srv http.Handler
	err error
}

// UIDataTransform is an optional dependency that allows the agent to add
// additional data into the UI index as needed. For example we use this to
// inject enterprise-only feature flags into the template without making this
// package inherently dependent on Enterprise-only code.
//
// It is passed the current RuntimeConfig being applied and a map containing the
// current data that will be passed to the template. It should be modified
// directly to inject additional context.
type UIDataTransform func(cfg *config.RuntimeConfig, data map[string]interface{}) error

// NewHandler returns a Handler that can be used to serve UI http requests. It
// accepts a full agent config since properties like ACLs being enabled affect
// the UI so we need more than just UIConfig parts.
func NewHandler(agentCfg *config.RuntimeConfig, logger hclog.Logger, transform UIDataTransform) *Handler {
	h := &Handler{
		logger:    logger.Named(logging.UIServer),
		transform: transform,
	}
	// Don't return the error since this is likely the result of a
	// misconfiguration and reloading config could fix it. Instead we'll capture
	// it and return an error for all calls to ServeHTTP so the misconfiguration
	// is visible. Sadly we can't log effectively
	if err := h.ReloadConfig(agentCfg); err != nil {
		h.state.Store(reloadableState{
			err: err,
		})
	}
	return h
}

// ServeHTTP implements http.Handler and serves UI HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// We need to support the path being trimmed by http.StripTags just like the
	// file servers do since http.StripPrefix will remove the leading slash in our
	// current config. Everything else works fine that way so we should to.
	pathTrimmed := strings.TrimLeft(r.URL.Path, "/")
	if pathTrimmed == compiledProviderJSPath {
		h.serveUIMetricsProviders(w, r)
		return
	}

	s := h.getState()
	if s == nil {
		panic("nil state")
	}
	if s.err != nil {
		http.Error(w, "UI server is misconfigured.", http.StatusInternalServerError)
		h.logger.Error("Failed to configure UI server: %s", s.err)
		return
	}
	s.srv.ServeHTTP(w, r)
}

// ReloadConfig is called by the agent when the configuration is reloaded and
// updates the UIConfig values the handler uses to serve requests.
func (h *Handler) ReloadConfig(newCfg *config.RuntimeConfig) error {
	newState := reloadableState{
		cfg: &newCfg.UIConfig,
	}

	var fs http.FileSystem

	if newCfg.UIConfig.Dir == "" {
		// Serve from assetFS
		fs = assetFS()
	} else {
		fs = http.Dir(newCfg.UIConfig.Dir)
	}

	// Render a new index.html with the new config values ready to serve.
	buf, info, err := h.renderIndex(newCfg, fs)
	if _, ok := err.(*os.PathError); ok && newCfg.UIConfig.Dir != "" {
		// A Path error indicates that there is no index.html. This could happen if
		// the user configured their own UI dir and is serving something that is not
		// our usual UI. This won't work perfectly because our uiserver will still
		// redirect everything to the UI but we shouldn't fail the entire UI server
		// with a 500 in this case. Partly that's just bad UX and partly it's a
		// breaking change although quite an edge case. Instead, continue but just
		// return a 404 response for the index.html and log a warning.
		h.logger.Warn("ui_config.dir does not contain an index.html. Index templating and redirects to index.html are disabled.")
	} else if err != nil {
		return err
	}

	// buf can be nil in the PathError case above. We should skip this part but
	// still serve the rest of the files in that case.
	if buf != nil {
		// Create a new fs that serves the rendered index file or falls back to the
		// underlying FS.
		fs = &bufIndexFS{
			fs:            fs,
			indexRendered: buf,
			indexInfo:     info,
		}

		// Wrap the buffering FS our redirect FS. This needs to happen later so that
		// redirected requests for /index.html get served the rendered version not the
		// original.
		fs = &redirectFS{fs: fs}
	}

	newState.srv = http.FileServer(fs)

	// Store the new state
	h.state.Store(newState)
	return nil
}

// getState is a helper to access the atomic internal state
func (h *Handler) getState() *reloadableState {
	if cfg, ok := h.state.Load().(reloadableState); ok {
		return &cfg
	}
	return nil
}

func (h *Handler) serveUIMetricsProviders(resp http.ResponseWriter, req *http.Request) {
	// Reload config in case it's changed
	state := h.getState()

	if len(state.cfg.MetricsProviderFiles) < 1 {
		http.Error(resp, "No provider JS files configured", http.StatusNotFound)
		return
	}

	var buf bytes.Buffer

	// Open each one and concatenate them
	for _, file := range state.cfg.MetricsProviderFiles {
		if err := concatFile(&buf, file); err != nil {
			http.Error(resp, "Internal Server Error", http.StatusInternalServerError)
			h.logger.Error("failed serving metrics provider js file", "file", file, "error", err)
			return
		}
	}
	// Done!
	resp.Header()["Content-Type"] = []string{"application/javascript"}
	_, err := buf.WriteTo(resp)
	if err != nil {
		http.Error(resp, "Internal Server Error", http.StatusInternalServerError)
		h.logger.Error("failed writing ui metrics provider files: %s", err)
		return
	}
}

func concatFile(buf *bytes.Buffer, file string) error {
	base := path.Base(file)
	_, err := buf.WriteString("// " + base + "\n\n")
	if err != nil {
		return fmt.Errorf("failed writing provider JS files: %w", err)
	}

	// Attempt to open the file
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed opening ui metrics provider JS file: %w", err)
	}
	defer f.Close()
	_, err = buf.ReadFrom(f)
	if err != nil {
		return fmt.Errorf("failed reading ui metrics provider JS file: %w", err)
	}
	_, err = buf.WriteString("\n\n")
	if err != nil {
		return fmt.Errorf("failed writing provider JS files: %w", err)
	}
	return nil
}

func (h *Handler) renderIndex(cfg *config.RuntimeConfig, fs http.FileSystem) ([]byte, os.FileInfo, error) {
	// Open the original index.html
	f, err := fs.Open("/index.html")
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed reading index.html: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("failed reading metadata for index.html: %w", err)
	}

	// Create template data from the current config.
	tplData, err := uiTemplateDataFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed loading UI config for template: %w", err)
	}

	// Allow caller to apply additional data transformations if needed.
	if h.transform != nil {
		if err := h.transform(cfg, tplData); err != nil {
			return nil, nil, fmt.Errorf("failed running transform: %w", err)
		}
	}

	tpl, err := template.New("index").Funcs(template.FuncMap{
		"jsonEncode": func(data map[string]interface{}) (string, error) {
			bs, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return "", fmt.Errorf("failed jsonEncode: %w", err)
			}
			return string(bs), nil
		},
	}).Parse(string(content))
	if err != nil {
		return nil, nil, fmt.Errorf("failed parsing index.html template: %w", err)
	}

	var buf bytes.Buffer

	err = tpl.Execute(&buf, tplData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to render index.html: %w", err)
	}

	return buf.Bytes(), info, nil
}
