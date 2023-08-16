// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package controller provides an API for implementing control loops on top of
// Consul resources. It is heavily inspired by [Kubebuilder] and the Kubernetes
// [controller runtime].
//
// [Kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
// [controller runtime]: https://github.com/kubernetes-sigs/controller-runtime
package controller
