// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package uiserver

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"text/template"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/logging"
)

const (
	compiledProviderJSPath = "assets/compiled-metrics-providers.js"
)

//go:embed dist
var dist embed.FS

// Handler is the http.Handler that serves the Consul UI. It may serve from the
// embedded fs.FS or from an external directory. It provides a few important
// transformations on the index.html file and includes a proxy for metrics
// backends.
type Handler struct {
	// runtimeConfig is a struct accessed through an atomic value to make
	// it safe to reload at run time. Each call to ServeHTTP will see the latest
	// version of the state without internal locking needed.
	runtimeConfig atomic.Value
	logger        hclog.Logger
	transform     UIDataTransform
}

// UIDataTransform is an optional dependency that allows the agent to add
// additional data into the UI index as needed. For example we use this to
// inject enterprise-only feature flags into the template without making this
// package inherently dependent on Enterprise-only code.
//
// It is passed the current RuntimeConfig being applied and a map containing the
// current data that will be passed to the template. It should be modified
// directly to inject additional context.
type UIDataTransform func(data map[string]interface{}) error

// NewHandler returns a Handler that can be used to serve UI http requests. It
// accepts a full agent config since properties like ACLs being enabled affect
// the UI so we need more than just UIConfig parts.
func NewHandler(runtimeCfg *config.RuntimeConfig, logger hclog.Logger, transform UIDataTransform) *Handler {
	h := &Handler{
		logger:    logger.Named(logging.UIServer),
		transform: transform,
	}
	h.runtimeConfig.Store(runtimeCfg)
	return h
}

// ServeHTTP implements http.Handler and serves UI HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// We need to support the path being trimmed by http.StripTags just like the
	// file servers do since http.StripPrefix will remove the leading slash in our
	// current config. Everything else works fine that way so we should to.
	pathTrimmed := strings.TrimLeft(r.URL.Path, "/")
	if pathTrimmed == compiledProviderJSPath {
		h.serveUIMetricsProviders(w)
		return
	}

	srv, err := h.handleIndex()
	if err != nil {
		http.Error(w, "UI server is misconfigured.", http.StatusInternalServerError)
		h.logger.Error("Failed to configure UI server: %s", err)
		return
	}
	srv.ServeHTTP(w, r)
}

// ReloadConfig is called by the agent when the configuration is reloaded and
// updates the UIConfig values the handler uses to serve requests.
func (h *Handler) ReloadConfig(newCfg *config.RuntimeConfig) error {
	h.runtimeConfig.Store(newCfg)
	return nil
}

func (h *Handler) handleIndex() (http.Handler, error) {
	cfg := h.getRuntimeConfig()

	var fsys fs.FS
	if cfg.UIConfig.Dir == "" {
		// strip the dist/ prefix
		sub, err := fs.Sub(dist, "dist")
		if err != nil {
			return nil, err
		}
		fsys = sub
	} else {
		fsys = os.DirFS(cfg.UIConfig.Dir)
	}

	// Render a new index.html with the new config values ready to serve.
	buf, err := h.renderIndexFile(cfg, fsys)
	if _, ok := err.(*os.PathError); ok && cfg.UIConfig.Dir != "" {
		// A Path error indicates that there is no index.html. This could happen if
		// the user configured their own UI dir and is serving something that is not
		// our usual UI. This won't work perfectly because our uiserver will still
		// redirect everything to the UI but we shouldn't fail the entire UI server
		// with a 500 in this case. Partly that's just bad UX and partly it's a
		// breaking change although quite an edge case. Instead, continue but just
		// return a 404 response for the index.html and log a warning.
		h.logger.Warn("ui_config.dir does not contain an index.html. Index templating and redirects to index.html are disabled.")
		return http.FileServer(http.FS(fsys)), nil
	}
	if err != nil {
		return nil, err
	}

	// Create a new fsys that serves the rendered index file or falls back to the
	// underlying FS.
	fsys = &bufIndexFS{
		fs:       fsys,
		bufIndex: buf,
	}

	// Wrap the buffering FS our redirect FS. This needs to happen later so that
	// redirected requests for /index.html get served the rendered version not the
	// original.
	return http.FileServer(http.FS(&redirectFS{fs: fsys})), nil
}

// getRuntimeConfig is a helper to atomically access the runtime config.
func (h *Handler) getRuntimeConfig() *config.RuntimeConfig {
	if cfg, ok := h.runtimeConfig.Load().(*config.RuntimeConfig); ok {
		return cfg
	}
	return nil
}

func (h *Handler) serveUIMetricsProviders(resp http.ResponseWriter) {
	// Reload config in case it's changed
	cfg := h.getRuntimeConfig()

	if len(cfg.UIConfig.MetricsProviderFiles) < 1 {
		http.Error(resp, "No provider JS files configured", http.StatusNotFound)
		return
	}

	var buf bytes.Buffer

	// Open each one and concatenate them
	for _, file := range cfg.UIConfig.MetricsProviderFiles {
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

func (h *Handler) renderIndexFile(cfg *config.RuntimeConfig, fsys fs.FS) (fs.File, error) {
	// Open the original index.html
	f, err := fsys.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed reading index.html: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed reading metadata for index.html: %w", err)
	}

	// Create template data from the current config.
	tplData, err := uiTemplateDataFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed loading UI config for template: %w", err)
	}

	// Allow caller to apply additional data transformations if needed.
	if h.transform != nil {
		if err := h.transform(tplData); err != nil {
			return nil, fmt.Errorf("failed running transform: %w", err)
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
		return nil, fmt.Errorf("failed parsing index.html template: %w", err)
	}

	var buf bytes.Buffer

	err = tpl.Execute(&buf, tplData)
	if err != nil {
		return nil, fmt.Errorf("failed to render index.html: %w", err)
	}

	file := newBufferedFile(&buf, info)
	return file, nil
}
