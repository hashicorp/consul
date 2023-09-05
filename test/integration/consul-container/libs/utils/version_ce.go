// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package utils

const (
	defaultImageName   = DefaultImageNameCE
	ImageVersionSuffix = ""
	isInEnterpriseRepo = false
)
