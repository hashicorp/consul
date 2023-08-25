// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows
// +build !windows

package checks

const DefaultDockerHost = "unix:///var/run/docker.sock"
