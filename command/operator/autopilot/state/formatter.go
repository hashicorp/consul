// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/consul/api"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

// Formatter defines methods provided by an autopilot state output formatter
type Formatter interface {
	FormatState(state *api.AutopilotState) (string, error)
}

// GetSupportedFormats returns supported formats
func GetSupportedFormats() []string {
	return []string{PrettyFormat, JSONFormat}
}

// NewFormatter returns Formatter implementation
func NewFormatter(format string) (formatter Formatter, err error) {
	switch format {
	case PrettyFormat:
		formatter = newPrettyFormatter()
	case JSONFormat:
		formatter = newJSONFormatter()
	default:
		err = fmt.Errorf("Unknown format: %s", format)
	}

	return formatter, err
}

func newPrettyFormatter() Formatter {
	return &prettyFormatter{}
}

type prettyFormatter struct {
}

func outputStringSlice(buffer *bytes.Buffer, indent string, values []string) {
	for _, val := range values {
		fmt.Fprintf(buffer, "%s%s\n", indent, val)
	}
}

type mapOutput struct {
	key   string
	value string
}

func formatZone(zoneName string, zone *api.AutopilotZone) string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "   %s:\n", zoneName)
	fmt.Fprintf(&buffer, "      Failure Tolerance: %d\n", zone.FailureTolerance)
	buffer.WriteString("      Voters:\n")
	outputStringSlice(&buffer, "         ", zone.Voters)
	buffer.WriteString("      Servers:\n")
	outputStringSlice(&buffer, "         ", zone.Servers)

	return buffer.String()
}

func formatServer(srv *api.AutopilotServer) string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "   %s\n", srv.ID)
	fmt.Fprintf(&buffer, "      Name:            %s\n", srv.Name)
	fmt.Fprintf(&buffer, "      Address:         %s\n", srv.Address)
	fmt.Fprintf(&buffer, "      Version:         %s\n", srv.Version)
	fmt.Fprintf(&buffer, "      Status:          %s\n", srv.Status)
	fmt.Fprintf(&buffer, "      Node Type:       %s\n", srv.NodeType)
	fmt.Fprintf(&buffer, "      Node Status:     %s\n", srv.NodeStatus)
	fmt.Fprintf(&buffer, "      Healthy:         %t\n", srv.Healthy)
	fmt.Fprintf(&buffer, "      Last Contact:    %s\n", srv.LastContact.String())
	fmt.Fprintf(&buffer, "      Last Term:       %d\n", srv.LastTerm)
	fmt.Fprintf(&buffer, "      Last Index:      %d\n", srv.LastIndex)
	if srv.RedundancyZone != "" {
		fmt.Fprintf(&buffer, "      Redundancy Zone: %s\n", srv.RedundancyZone)
	}
	if srv.UpgradeVersion != "" {
		fmt.Fprintf(&buffer, "      Upgrade Version: %s\n", srv.UpgradeVersion)
	}
	if srv.ReadReplica {
		fmt.Fprintf(&buffer, "      Read Replica:    %t\n", srv.ReadReplica)
	}
	if len(srv.Meta) > 0 {
		buffer.WriteString("      Meta\n")
		var outputs []mapOutput
		for k, v := range srv.Meta {
			outputs = append(outputs, mapOutput{key: k, value: fmt.Sprintf("         %q: %q\n", k, v)})
		}

		sort.Slice(outputs, func(i, j int) bool {
			return outputs[i].key < outputs[j].key
		})

		for _, output := range outputs {
			buffer.WriteString(output.value)
		}
	}

	return buffer.String()
}

func (f *prettyFormatter) FormatState(state *api.AutopilotState) (string, error) {

	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "Healthy:                      %t\n", state.Healthy)
	fmt.Fprintf(&buffer, "Failure Tolerance:            %d\n", state.FailureTolerance)
	fmt.Fprintf(&buffer, "Optimistic Failure Tolerance: %d\n", state.OptimisticFailureTolerance)
	fmt.Fprintf(&buffer, "Leader:                       %s\n", state.Leader)
	buffer.WriteString("Voters:\n")
	outputStringSlice(&buffer, "   ", state.Voters)

	if len(state.ReadReplicas) > 0 {
		buffer.WriteString("Read Replicas:\n")
		outputStringSlice(&buffer, "   ", state.ReadReplicas)
	}

	if len(state.RedundancyZones) > 0 {
		var outputs []mapOutput
		buffer.WriteString("Redundancy Zones:\n")
		for zoneName, zone := range state.RedundancyZones {
			outputs = append(outputs, mapOutput{key: zoneName, value: formatZone(zoneName, &zone)})
		}
		sort.Slice(outputs, func(i, j int) bool {
			return outputs[i].key < outputs[j].key
		})

		for _, output := range outputs {
			buffer.WriteString(output.value)
		}
	}

	if state.Upgrade != nil {
		u := state.Upgrade
		buffer.WriteString("Upgrade:\n")
		fmt.Fprintf(&buffer, "   Status:         %s\n", u.Status)
		fmt.Fprintf(&buffer, "   Target Version: %s\n", u.TargetVersion)
		if len(u.TargetVersionVoters) > 0 {
			buffer.WriteString("   Target Version Voters:\n")
			outputStringSlice(&buffer, "      ", u.TargetVersionVoters)
		}
		if len(u.TargetVersionNonVoters) > 0 {
			buffer.WriteString("   Target Version Non-Voters:\n")
			outputStringSlice(&buffer, "      ", u.TargetVersionNonVoters)
		}
		if len(u.TargetVersionReadReplicas) > 0 {
			buffer.WriteString("   Target Version ReadReplicas:\n")
			outputStringSlice(&buffer, "      ", u.TargetVersionReadReplicas)
		}
		if len(u.OtherVersionVoters) > 0 {
			buffer.WriteString("   Other Version Voters:\n")
			outputStringSlice(&buffer, "      ", u.OtherVersionVoters)
		}
		if len(u.OtherVersionNonVoters) > 0 {
			buffer.WriteString("   Other Version Non-Voters:\n")
			outputStringSlice(&buffer, "      ", u.OtherVersionNonVoters)
		}
		if len(u.OtherVersionReadReplicas) > 0 {
			buffer.WriteString("   Other Version ReadReplicas:\n")
			outputStringSlice(&buffer, "      ", u.OtherVersionReadReplicas)
		}
	}

	buffer.WriteString("Servers:\n")
	var outputs []mapOutput
	for id, srv := range state.Servers {
		outputs = append(outputs, mapOutput{key: id, value: formatServer(&srv)})
	}

	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].key < outputs[j].key
	})

	for _, output := range outputs {
		buffer.WriteString(output.value)
	}

	return buffer.String(), nil
}

func newJSONFormatter() Formatter {
	return &jsonFormatter{}
}

type jsonFormatter struct {
}

func (f *jsonFormatter) FormatState(state *api.AutopilotState) (string, error) {
	b, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal token: %v", err)
	}
	return string(b), nil
}
