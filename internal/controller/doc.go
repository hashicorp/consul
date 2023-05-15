// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package controller provides an API for implementing control loops on top of
// Consul resources. It is heavily inspired by [Kubebuilder] and the Kubernetes
// [controller runtime].
//
// [Kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
// [controller runtime]: https://github.com/kubernetes-sigs/controller-runtime
package controller
