package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

type Formatter interface {
	Format(OutputFormat) (string, error)
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
	return prettyFormatter{}
}

func (_ *prettyFormatter) Format(info *OutputFormat) (string, error) {
	var buffer bytes.Buffer

	// For the original snapshot inspect information
	tw := tabwriter.NewWriter(&buffer, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "ID\t%s\n", info.Meta.ID)
	fmt.Fprintf(tw, "Size\t%d\n", info.Meta.Size)
	fmt.Fprintf(tw, "Index\t%d\n", info.Meta.Index)
	fmt.Fprintf(tw, "Term\t%d\n", info.Meta.Term)
	fmt.Fprintf(tw, "Version\t%d\n", info.Meta.Version)

	// For the enhanced stats
	ss := make([]typeStats, 0, len(info.Stats))

	for _, s := range info.Stats {
		ss = append(ss, s)
	}

	// Sort the stat slice
	sort.Slice(ss, func(i, j int) bool { return ss[i].Sum > ss[j].Sum })

	tw = tabwriter.NewWriter(&buffer, 8, 8, 6, ' ', 0)
	fmt.Fprintf(tw, "\n Type\tCount\tSize\t")
	fmt.Fprintf(tw, " %s\t%s\t%s\t", "----", "----", "----")
	for _, s := range ss {
		fmt.Fprintf(fmt.Sprintf(tw, "\n %s\t%d\t%s\t", s.Name, s.Count, ByteSize(uint64(s.Sum))))
	}
	fmt.Fprintf(tw, "\n %s\t%s\t%s\t", "----", "----", "----")
	fmt.Fprintf(tw, "\n Total\t\t%s\t", ByteSize(uint64(offset)))
	return buffer.String(), nil
}

type jsonFormatter struct{}

func newJSONFormatter() Formatter {
	return &jsonFormatter{}
}

func (_ *jsonFormatter) FormattFormat(info *OutputFormat) (string, error) {
	b, err := json.MarshalIndent(info.Meta, "", "   ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal original snapshot stats: %v", err)
	}

	ss := make([]typeStats, 0, len(info.Stats))
	for _, s := range info.Stats {
		ss = append(ss, s)
	}
	// Sort the stat slice
	sort.Slice(ss, func(i, j int) bool { return ss[i].Sum > ss[j].Sum })

	b, err = json.MarshalIndent(ss, "", "   ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal enhanced snapshot stats: %v", err)
	}
	return string(b), nil
}

const (
	BYTE = 1 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
)

func ByteSize(bytes uint64) string {
	unit := ""
	value := float64(bytes)

	switch {
	case bytes >= TERABYTE:
		unit = "TB"
		value = value / TERABYTE
	case bytes >= GIGABYTE:
		unit = "GB"
		value = value / GIGABYTE
	case bytes >= MEGABYTE:
		unit = "MB"
		value = value / MEGABYTE
	case bytes >= KILOBYTE:
		unit = "KB"
		value = value / KILOBYTE
	case bytes >= BYTE:
		unit = "B"
	case bytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}
