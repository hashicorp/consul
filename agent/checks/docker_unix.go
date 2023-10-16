// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package checks

const DefaultDockerHost = "unix:///var/run/docker.sock"
