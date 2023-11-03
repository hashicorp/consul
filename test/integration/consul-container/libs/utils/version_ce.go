// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package utils

const (
	defaultImageName   = DefaultImageNameCE
	ImageVersionSuffix = ""
	isInEnterpriseRepo = false
)
