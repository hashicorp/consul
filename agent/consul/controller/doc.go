// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package controller contains a re-implementation of the Kubernetes
// [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
// with the core using Consul's event publishing pipeline rather than
// Kubernetes' client list/watch APIs.
//
// Generally this package enables defining asynchronous control loops
// meant to be run on a Consul cluster's leader that reconcile derived state
// in config entries that might be dependent on multiple sources.

package controller
