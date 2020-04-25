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

	buffer.WriteString(fmt.Sprintf("Name:         %s\n", method.Name))
	buffer.WriteString(fmt.Sprintf("Type:         %s\n", method.Type))
	if method.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:    %s\n", method.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("Description:  %s\n", method.Description))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("Create Index: %d\n", method.CreateIndex))
		buffer.WriteString(fmt.Sprintf("Modify Index: %d\n", method.ModifyIndex))
	}
	buffer.WriteString(fmt.Sprintln("Config:"))
	output, err := json.MarshalIndent(method.Config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("Error formatting auth method configuration: %s", err)
	}
	buffer.WriteString(string(output))

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

	buffer.WriteString(fmt.Sprintf("%s:\n", method.Name))
	buffer.WriteString(fmt.Sprintf("   Type:         %s\n", method.Type))
	if method.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("   Namespace:    %s\n", method.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("   Description:  %s\n", method.Description))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("   Create Index: %d\n", method.CreateIndex))
		buffer.WriteString(fmt.Sprintf("   Modify Index: %d\n", method.ModifyIndex))
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
