package nametemplate

import (
	"errors"
	"strings"

	dns_strings "github.com/miekg/coredns/middleware/pkg/strings"
)

// Likely symbols that require support:
// {id}
// {ip}
// {portname}
// {protocolname}
// {servicename}
// {namespace}
// {type}              "svc" or "pod"
// {zone}

// SkyDNS normal services have an A-record of the form "{servicename}.{namespace}.{type}.{zone}"
// This resolves to the cluster IP of the service.

// SkyDNS headless services have an A-record of the form "{servicename}.{namespace}.{type}.{zone}"
// This resolves to the set of IPs of the pods selected by the Service. Clients are expected to
// consume the set or else use round-robin selection from the set.

var symbols = map[string]string{
	"service":   "{service}",
	"namespace": "{namespace}",
	"type":      "{type}",
	"zone":      "{zone}",
}

var types = []string{
	"svc",
	"pod",
}

var requiredSymbols = []string{
	"namespace",
	"service",
}

// TODO: Validate that provided NameTemplate string only contains:
//			* valid, known symbols, or
//			* static strings

// TODO: Support collapsing multiple segments into a symbol. Either:
//			* all left-over segments are used as the "service" name, or
//			* some scheme like "{namespace}.{namespace}" means use
//			  segments concatenated with a "." for the namespace, or
//			* {namespace2:4} means use segements 2->4 for the namespace.

// TODO: possibly need to store length of segmented format to handle cases
//       where query string segments to a shorter or longer list than the template.
//		 When query string segments to shorter than template:
//			* either wildcards are being used, or
//			* we are not looking up an A, AAAA, or SRV record (eg NS), or
//			* we can just short-circuit failure before hitting the k8s API.
//		 Where the query string is longer than the template, need to define which
//		 symbol consumes the other segments. Most likely this would be the servicename.
//		 Also consider how to handle static strings in the format template.

// Template holds the kubernetes template.
type Template struct {
	formatString string
	splitFormat  []string
	// Element is a map of element name :: index in the segmented record name for the named element
	Element map[string]int
}

// SetTemplate use the string s the set the template.
func (t *Template) SetTemplate(s string) error {
	var err error

	t.Element = map[string]int{}

	t.formatString = s
	t.splitFormat = strings.Split(t.formatString, ".")
	for templateIndex, v := range t.splitFormat {
		elementPositionSet := false
		for name, symbol := range symbols {
			if v == symbol {
				t.Element[name] = templateIndex
				elementPositionSet = true
				break
			}
		}
		if !elementPositionSet {
			if strings.Contains(v, "{") {
				err = errors.New("Record name template contains the unknown symbol '" + v + "'")
				return err
			}
		}
	}

	if err == nil && !t.IsValid() {
		err = errors.New("Record name template does not pass NameTemplate validation")
		return err
	}

	return err
}

// TODO: Find a better way to pull the data segments out of the
//       query string based on the template. Perhaps it is better
//		 to treat the query string segments as a reverse stack and
//       step down the stack to find the right element.

// ZoneFromSegmentArray returns the zone string from the segments.
func (t *Template) ZoneFromSegmentArray(segments []string) string {
	index, ok := t.Element["zone"]
	if !ok {
		return ""
	}
	return strings.Join(segments[index:], ".")
}

// NamespaceFromSegmentArray returns the namespace string from the segments.
func (t *Template) NamespaceFromSegmentArray(segments []string) string {
	return t.symbolFromSegmentArray("namespace", segments)
}

// ServiceFromSegmentArray returns the service string from the segments.
func (t *Template) ServiceFromSegmentArray(segments []string) string {
	return t.symbolFromSegmentArray("service", segments)
}

// TypeFromSegmentArray returns the type string from the segments.
func (t *Template) TypeFromSegmentArray(segments []string) string {
	typeSegment := t.symbolFromSegmentArray("type", segments)

	// Limit type to known types symbols
	if dns_strings.StringInSlice(typeSegment, types) {
		return ""
	}

	return typeSegment
}

func (t *Template) symbolFromSegmentArray(symbol string, segments []string) string {
	index, ok := t.Element[symbol]
	if !ok {
		return ""
	}
	return segments[index]
}

// RecordNameFromNameValues returns the string produced by applying the
// values to the NameTemplate format string.
func (t *Template) RecordNameFromNameValues(values NameValues) string {
	recordName := make([]string, len(t.splitFormat))
	copy(recordName[:], t.splitFormat)

	for name, index := range t.Element {
		if index == -1 {
			continue
		}
		switch name {
		case "type":
			recordName[index] = values.TypeName
		case "service":
			recordName[index] = values.ServiceName
		case "namespace":
			recordName[index] = values.Namespace
		case "zone":
			recordName[index] = values.Zone
		}
	}
	return strings.Join(recordName, ".")
}

// IsValid returns true if the template has all the required symbols, false otherwise.
func (t *Template) IsValid() bool {
	result := true

	// Ensure that all requiredSymbols are found in NameTemplate
	for _, symbol := range requiredSymbols {
		if _, ok := t.Element[symbol]; !ok {
			result = false
			break
		}
	}

	return result
}

// NameValues contains a number of values.
// TODO(...): better docs.
type NameValues struct {
	ServiceName string
	Namespace   string
	TypeName    string
	Zone        string
}
