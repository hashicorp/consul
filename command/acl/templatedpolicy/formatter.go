// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package templatedpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/consul/api"
)

const (
	PrettyFormat     string = "pretty"
	JSONFormat       string = "json"
	WhitespaceIndent        = "\t"
)

// Formatter defines methods provided by templated-policy command output formatter
type Formatter interface {
	FormatTemplatedPolicy(policy api.ACLTemplatedPolicyResponse) (string, error)
	FormatTemplatedPolicyList(policies map[string]api.ACLTemplatedPolicyResponse) (string, error)
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
		err = fmt.Errorf("unknown format: %q", format)
	}

	return formatter, err
}

func newPrettyFormatter(showMeta bool) Formatter {
	return &prettyFormatter{showMeta}
}

func newJSONFormatter(showMeta bool) Formatter {
	return &jsonFormatter{showMeta}
}

type prettyFormatter struct {
	showMeta bool
}

// FormatTemplatedPolicy displays template name, input variables and example usages. When
// showMeta is true, we display raw template code and schema.
// This implementation is a conscious choice as  we know builtin variables we know every required/optional input variables
// so we can just hardcode this.
// In the future, when we implement user defined templated policies, we will move this to some sort of schema parsing.
// This implementation allows us to move forward without limiting ourselves when implementing user defined templated policies.
func (f *prettyFormatter) FormatTemplatedPolicy(templatedPolicy api.ACLTemplatedPolicyResponse) (string, error) {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("Name:            %s\n", templatedPolicy.TemplateName))
	buffer.WriteString(fmt.Sprintf("Description:     %s\n", templatedPolicy.Description))

	buffer.WriteString("Input variables:")
	switch templatedPolicy.TemplateName {
	case api.ACLTemplatedPolicyServiceName:
		nameRequiredVariableOutput(&buffer, templatedPolicy.TemplateName, "The name of the service", "api")
	case api.ACLTemplatedPolicyNodeName:
		nameRequiredVariableOutput(&buffer, templatedPolicy.TemplateName, "The node name", "node-1")
	case api.ACLTemplatedPolicyAPIGatewayName:
		nameRequiredVariableOutput(&buffer, templatedPolicy.TemplateName, "The api gateway service name", "api-gateway")
	case api.ACLTemplatedPolicyDNSName, api.ACLTemplatedPolicyNomadServerName, api.ACLTemplatedPolicyNomadClientName:
		noRequiredVariablesOutput(&buffer, templatedPolicy.TemplateName)
	default:
		buffer.WriteString("   None\n")
	}

	if f.showMeta {
		if templatedPolicy.Schema != "" {
			buffer.WriteString(fmt.Sprintf("Schema:\n%s\n\n", templatedPolicy.Schema))
		}
		buffer.WriteString(fmt.Sprintf("Raw Template:\n%s\n", templatedPolicy.Template))
	}

	return buffer.String(), nil
}

func noRequiredVariablesOutput(buffer *bytes.Buffer, templateName string) {
	buffer.WriteString(" None\n")
	buffer.WriteString("Example usage:\n")
	buffer.WriteString(fmt.Sprintf("%sconsul acl token create -templated-policy %s\n", WhitespaceIndent, templateName))
}

func nameRequiredVariableOutput(buffer *bytes.Buffer, templateName, description, exampleName string) {
	buffer.WriteString(fmt.Sprintf("\n%sName: String - Required - %s.\n", WhitespaceIndent, description))
	buffer.WriteString("Example usage:\n")
	buffer.WriteString(fmt.Sprintf("%sconsul acl token create -templated-policy %s -var name:%s\n", WhitespaceIndent, templateName, exampleName))
}

func (f *prettyFormatter) FormatTemplatedPolicyList(policies map[string]api.ACLTemplatedPolicyResponse) (string, error) {
	var buffer bytes.Buffer

	templateNames := make([]string, 0, len(policies))
	for _, templatedPolicy := range policies {
		templateNames = append(templateNames, templatedPolicy.TemplateName)
	}

	//ensure the list is consistently sorted by strings
	sort.Strings(templateNames)
	for _, name := range templateNames {
		buffer.WriteString(fmt.Sprintf("%s\n", name))
	}

	return buffer.String(), nil
}

type jsonFormatter struct {
	showMeta bool
}

func (f *jsonFormatter) FormatTemplatedPolicy(templatedPolicy api.ACLTemplatedPolicyResponse) (string, error) {
	b, err := json.MarshalIndent(templatedPolicy, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal templated policy: %v", err)
	}
	return string(b), nil
}

func (f *jsonFormatter) FormatTemplatedPolicyList(templatedPolicies map[string]api.ACLTemplatedPolicyResponse) (string, error) {
	b, err := json.MarshalIndent(templatedPolicies, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal templated policies: %v", err)
	}
	return string(b), nil
}
