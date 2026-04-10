// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authmethod

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/consul/api"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

// Formatter defines methods provided by authmethod command output formatter
type Formatter interface {
	FormatAuthMethod(method *api.ACLAuthMethod) (string, error)
	FormatAuthMethodList(methods []*api.ACLAuthMethodListEntry) (string, error)
}

// GetSupportedFormats returns supported formats
func GetSupportedFormats() []string {
	return []string{PrettyFormat, JSONFormat}
}

// NewFormatter returns Formatter implementation
func NewFormatter(format string, showMeta bool) (formatter Formatter, err error) {
	switch format {
	case PrettyFormat:
		formatter = newPrettyFormatter(showMeta)
	case JSONFormat:
		formatter = newJSONFormatter(showMeta)
	default:
		err = fmt.Errorf("Unknown format: %s", format)
	}

	return formatter, err
}

func newPrettyFormatter(showMeta bool) Formatter {
	return &prettyFormatter{showMeta}
}

type prettyFormatter struct {
	showMeta bool
}

func (f *prettyFormatter) FormatAuthMethod(method *api.ACLAuthMethod) (string, error) {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "Name:          %s\n", method.Name)
	fmt.Fprintf(&buffer, "Type:          %s\n", method.Type)
	if method.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:     %s\n", method.Partition)
	}
	if method.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:     %s\n", method.Namespace)
	}
	if method.DisplayName != "" {
		fmt.Fprintf(&buffer, "DisplayName:   %s\n", method.DisplayName)
	}
	fmt.Fprintf(&buffer, "Description:   %s\n", method.Description)
	if method.MaxTokenTTL > 0 {
		fmt.Fprintf(&buffer, "MaxTokenTTL:   %s\n", method.MaxTokenTTL)
	}
	if method.TokenLocality != "" {
		fmt.Fprintf(&buffer, "TokenLocality: %s\n", method.TokenLocality)
	}
	if method.TokenNameFormat != "" {
		buffer.WriteString(fmt.Sprintf("TokenNameFormat: %s\n", method.TokenNameFormat))
	}
	if len(method.NamespaceRules) > 0 {
		fmt.Fprintln(&buffer, "NamespaceRules:")
		for _, rule := range method.NamespaceRules {
			fmt.Fprintf(&buffer, "   Selector:      %s\n", rule.Selector)
			fmt.Fprintf(&buffer, "   BindNamespace: %s\n", rule.BindNamespace)
		}
	}
	if f.showMeta {
		fmt.Fprintf(&buffer, "Create Index:  %d\n", method.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index:  %d\n", method.ModifyIndex)
	}
	fmt.Fprintln(&buffer, "Config:")
	output, err := json.MarshalIndent(method.Config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("Error formatting auth method configuration: %s", err)
	}
	buffer.Write(output)

	return buffer.String(), nil
}

func (f *prettyFormatter) FormatAuthMethodList(methods []*api.ACLAuthMethodListEntry) (string, error) {
	var buffer bytes.Buffer

	for _, method := range methods {
		buffer.WriteString(f.formatAuthMethodListEntry(method))
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) formatAuthMethodListEntry(method *api.ACLAuthMethodListEntry) string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "%s:\n", method.Name)
	fmt.Fprintf(&buffer, "   Type:         %s\n", method.Type)
	if method.Partition != "" {
		fmt.Fprintf(&buffer, "   Partition:    %s\n", method.Partition)
	}
	if method.Namespace != "" {
		fmt.Fprintf(&buffer, "   Namespace:    %s\n", method.Namespace)
	}
	if method.DisplayName != "" {
		fmt.Fprintf(&buffer, "   DisplayName:  %s\n", method.DisplayName)
	}
	fmt.Fprintf(&buffer, "   Description:  %s\n", method.Description)
	if f.showMeta {
		fmt.Fprintf(&buffer, "   Create Index: %d\n", method.CreateIndex)
		fmt.Fprintf(&buffer, "   Modify Index: %d\n", method.ModifyIndex)
	}

	return buffer.String()
}

func newJSONFormatter(showMeta bool) Formatter {
	return &jsonFormatter{showMeta}
}

type jsonFormatter struct {
	showMeta bool
}

func (f *jsonFormatter) FormatAuthMethod(method *api.ACLAuthMethod) (string, error) {
	b, err := json.MarshalIndent(method, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal authmethod:, %v", err)
	}
	return string(b), nil
}

func (f *jsonFormatter) FormatAuthMethodList(methods []*api.ACLAuthMethodListEntry) (string, error) {
	b, err := json.MarshalIndent(methods, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal authmethods:, %v", err)
	}
	return string(b), nil
}
