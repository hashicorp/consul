// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tfgen

import (
	"embed"
	"text/template"
)

//go:embed templates/container-app-dataplane.tf.tmpl
//go:embed templates/container-app-sidecar.tf.tmpl
//go:embed templates/container-app.tf.tmpl
//go:embed templates/container-consul.tf.tmpl
//go:embed templates/container-mgw.tf.tmpl
//go:embed templates/container-pause.tf.tmpl
//go:embed templates/container-proxy.tf.tmpl
//go:embed templates/container-coredns.tf.tmpl
var content embed.FS

var (
	tfAppDataplaneT = template.Must(template.ParseFS(content, "templates/container-app-dataplane.tf.tmpl"))
	tfAppSidecarT   = template.Must(template.ParseFS(content, "templates/container-app-sidecar.tf.tmpl"))
	tfAppT          = template.Must(template.ParseFS(content, "templates/container-app.tf.tmpl"))
	tfConsulT       = template.Must(template.ParseFS(content, "templates/container-consul.tf.tmpl"))
	tfMeshGatewayT  = template.Must(template.ParseFS(content, "templates/container-mgw.tf.tmpl"))
	tfPauseT        = template.Must(template.ParseFS(content, "templates/container-pause.tf.tmpl"))
	tfForwardProxyT = template.Must(template.ParseFS(content, "templates/container-proxy.tf.tmpl"))
	tfCorednsT      = template.Must(template.ParseFS(content, "templates/container-coredns.tf.tmpl"))
)
