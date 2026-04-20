// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package xds

func entDestinationPortClusterName(clusterName, _ string) string {
	return clusterName
}

func entDestinationPortALPN(_ string) []string {
	return nil
}

func entDestinationPortListenerName(baseName, _ string) string {
	return baseName
}
