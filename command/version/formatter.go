// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

type Formatter interface {
	Format(info *VersionInfo) (string, error)
}

func GetSupportedFormats() []string {
	return []string{PrettyFormat, JSONFormat}
}

func NewFormatter(format string) (Formatter, error) {
	switch format {
	case PrettyFormat:
		return newPrettyFormatter(), nil
	case JSONFormat:
		return newJSONFormatter(), nil
	default:
		return nil, fmt.Errorf("Unknown format: %s", format)
	}
}

type prettyFormatter struct{}

func newPrettyFormatter() Formatter {
	return &prettyFormatter{}
}

func (*prettyFormatter) Format(info *VersionInfo) (string, error) {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "Consul v%s\n", info.HumanVersion)
	if info.Revision != "" {
		fmt.Fprintf(&buffer, "Revision %s\n", info.Revision)
	}

	fmt.Fprintf(&buffer, "Build Date %s\n", info.BuildDate.Format(time.RFC3339))

	if info.FIPS != "" {
		fmt.Fprintf(&buffer, "FIPS: %s\n", info.FIPS)
	}

	var supplement string
	if info.RPC.Default < info.RPC.Max {
		supplement = fmt.Sprintf(" (agent will automatically use protocol >%d when speaking to compatible agents)",
			info.RPC.Default)
	}
	fmt.Fprintf(&buffer, "Protocol %d spoken by default, understands %d to %d%s\n",
		info.RPC.Default, info.RPC.Min, info.RPC.Max, supplement)

	return buffer.String(), nil
}

type jsonFormatter struct{}

func newJSONFormatter() Formatter {
	return &jsonFormatter{}
}

func (*jsonFormatter) Format(info *VersionInfo) (string, error) {
	b, err := json.MarshalIndent(info, "", "   ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal version info: %v", err)
	}
	return string(b), nil
}
