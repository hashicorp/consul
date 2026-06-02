// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"strings"
)

func destinationPortClusterName(clusterName, destinationPort string) string {
	if clusterName == "" {
		return ""
	}
	if destinationPort == "" {
		return clusterName
	}
	return entDestinationPortClusterName(clusterName, destinationPort)
}

func destinationPortALPN(destinationPort string) []string {
	if destinationPort == "" {
		return nil
	}
	return entDestinationPortALPN(destinationPort)
}

func destinationPortListenerName(baseName, destinationPort string) string {
	if destinationPort == "" {
		return baseName
	}

	name, _, _ := strings.Cut(baseName, "?")
	return entDestinationPortListenerName(name, destinationPort)
}
