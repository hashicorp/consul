// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tfgen

import (
	"embed"
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
