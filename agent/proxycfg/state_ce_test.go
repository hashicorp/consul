// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

func recordWatchesEnterprise(*stateConfig, *watchRecorder) {}
