// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package bindingrule

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

// Formatter defines methods provided by bindingrule command output formatter
type Formatter interface {
	FormatBindingRule(rule *api.ACLBindingRule) (string, error)
	FormatBindingRuleList(rules []*api.ACLBindingRule) (string, error)
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

func (f *prettyFormatter) FormatBindingRule(rule *api.ACLBindingRule) (string, error) {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "ID:           %s\n", rule.ID)
	if rule.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:    %s\n", rule.Partition)
	}
	if rule.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:    %s\n", rule.Namespace)
	}
	fmt.Fprintf(&buffer, "AuthMethod:   %s\n", rule.AuthMethod)
	fmt.Fprintf(&buffer, "Description:  %s\n", rule.Description)
	fmt.Fprintf(&buffer, "BindType:     %s\n", rule.BindType)
	fmt.Fprintf(&buffer, "BindName:     %s\n", rule.BindName)
	if rule.BindVars != nil && rule.BindVars.Name != "" {
		fmt.Fprintf(&buffer, "BindVars:     \n     - Name: %s\n", rule.BindVars.Name)
	}

	fmt.Fprintf(&buffer, "Selector:     %s\n", rule.Selector)
	if f.showMeta {
		fmt.Fprintf(&buffer, "Create Index: %d\n", rule.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index: %d\n", rule.ModifyIndex)
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) FormatBindingRuleList(rules []*api.ACLBindingRule) (string, error) {
	var buffer bytes.Buffer

	for _, rule := range rules {
		buffer.WriteString(f.formatBindingRuleListEntry(rule))
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) formatBindingRuleListEntry(rule *api.ACLBindingRule) string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "%s:\n", rule.ID)
	if rule.Partition != "" {
		fmt.Fprintf(&buffer, "   Partition:    %s\n", rule.Partition)
	}
	if rule.Namespace != "" {
		fmt.Fprintf(&buffer, "   Namespace:    %s\n", rule.Namespace)
	}
	fmt.Fprintf(&buffer, "   AuthMethod:   %s\n", rule.AuthMethod)
	fmt.Fprintf(&buffer, "   Description:  %s\n", rule.Description)
	fmt.Fprintf(&buffer, "   BindType:     %s\n", rule.BindType)
	fmt.Fprintf(&buffer, "   BindName:     %s\n", rule.BindName)
	if rule.BindVars != nil && rule.BindVars.Name != "" {
		fmt.Fprintf(&buffer, "   BindVars:     \n      - Name: %s\n", rule.BindVars.Name)
	}
	fmt.Fprintf(&buffer, "   Selector:     %s\n", rule.Selector)
	if f.showMeta {
		fmt.Fprintf(&buffer, "   Create Index: %d\n", rule.CreateIndex)
		fmt.Fprintf(&buffer, "   Modify Index: %d\n", rule.ModifyIndex)
	}

	return buffer.String()
}

func newJSONFormatter(showMeta bool) Formatter {
	return &jsonFormatter{showMeta}
}

type jsonFormatter struct {
	showMeta bool
}

func (f *jsonFormatter) FormatBindingRule(rule *api.ACLBindingRule) (string, error) {
	b, err := json.MarshalIndent(rule, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal binding rule:, %v", err)
	}
	return string(b), nil
}

func (f *jsonFormatter) FormatBindingRuleList(rules []*api.ACLBindingRule) (string, error) {
	b, err := json.MarshalIndent(rules, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal binding rules:, %v", err)

	}
	return string(b), nil
}
