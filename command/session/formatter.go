package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

type Formatter interface {
	FormatSession(s *api.SessionEntry) (string, error)
	FormatSessionList(s []*api.SessionEntry) (string, error)
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

func newPrettyFormatter() Formatter {
	return &prettyFormatter{}
}

type prettyFormatter struct{}

func (f *prettyFormatter) FormatSession(s *api.SessionEntry) (string, error) {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("ID:          %s\n", s.ID))
	if s.Name != "" {
		buffer.WriteString(fmt.Sprintf("Name:        %s\n", s.Name))
	}
	buffer.WriteString(fmt.Sprintf("Node:        %s\n", s.Node))
	buffer.WriteString(fmt.Sprintf("LockDelay:   %s\n", s.LockDelay.String()))
	buffer.WriteString(fmt.Sprintf("Behavior:    %s\n", s.Behavior))
	if s.TTL != "" {
		buffer.WriteString(fmt.Sprintf("TTL:         %s\n", s.TTL))
	}
	if s.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:   %s\n", s.Namespace))
	}
	if len(s.NodeChecks) != 0 {
		buffer.WriteString(fmt.Sprintf("Node Checks: %s\n", strings.Join(s.NodeChecks, ", ")))
	}
	if len(s.ServiceChecks) != 0 {
		buffer.WriteString("Service Checks:\n")

		for _, sc := range s.ServiceChecks {
			buffer.WriteString(fmt.Sprintf("  - ID: %s\n", sc.ID))
			if sc.Namespace != "" {
				buffer.WriteString(fmt.Sprintf("    Namespace: %s\n", sc.Namespace))
			}
		}
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) FormatSessionList(s []*api.SessionEntry) (string, error) {
	var buffer bytes.Buffer

	for i, session := range s {
		str, err := f.FormatSession(session)
		if err != nil {
			return "", err
		}
		buffer.WriteString(str)

		if i != len(s)-1 {
			buffer.WriteString("\n")
		}
	}
	return buffer.String(), nil
}

func newJSONFormatter() Formatter {
	return &jsonFormatter{}
}

type jsonFormatter struct{}

func (f *jsonFormatter) FormatSession(s *api.SessionEntry) (string, error) {
	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal session: %s", err)
	}
	return string(b), err
}

func (f *jsonFormatter) FormatSessionList(s []*api.SessionEntry) (string, error) {
	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal sessions: %s", err)
	}
	return string(b), nil
}
