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

	buffer.WriteString(fmt.Sprintf("ID:           %s\n", rule.ID))
	if rule.Partition != "" {
		buffer.WriteString(fmt.Sprintf("Partition:    %s\n", rule.Partition))
	}
	if rule.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:    %s\n", rule.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("AuthMethod:   %s\n", rule.AuthMethod))
	buffer.WriteString(fmt.Sprintf("Description:  %s\n", rule.Description))
	buffer.WriteString(fmt.Sprintf("BindType:     %s\n", rule.BindType))
	buffer.WriteString(fmt.Sprintf("BindName:     %s\n", rule.BindName))
	buffer.WriteString(fmt.Sprintf("Selector:     %s\n", rule.Selector))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("Create Index: %d\n", rule.CreateIndex))
		buffer.WriteString(fmt.Sprintf("Modify Index: %d\n", rule.ModifyIndex))
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

	buffer.WriteString(fmt.Sprintf("%s:\n", rule.ID))
	if rule.Partition != "" {
		buffer.WriteString(fmt.Sprintf("   Partition:    %s\n", rule.Partition))
	}
	if rule.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("   Namespace:    %s\n", rule.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("   AuthMethod:   %s\n", rule.AuthMethod))
	buffer.WriteString(fmt.Sprintf("   Description:  %s\n", rule.Description))
	buffer.WriteString(fmt.Sprintf("   BindType:     %s\n", rule.BindType))
	buffer.WriteString(fmt.Sprintf("   BindName:     %s\n", rule.BindName))
	buffer.WriteString(fmt.Sprintf("   Selector:     %s\n", rule.Selector))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("   Create Index: %d\n", rule.CreateIndex))
		buffer.WriteString(fmt.Sprintf("   Modify Index: %d\n", rule.ModifyIndex))
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
