// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package utils

const (
	defaultImageName   = DefaultImageNameOSS
	ImageVersionSuffix = ""
	isInEnterpriseRepo = false
)
