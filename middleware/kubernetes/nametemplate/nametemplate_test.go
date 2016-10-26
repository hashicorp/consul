package nametemplate

import (
	"strings"
	"testing"
)

const (
	zone      = 0
	namespace = 1
	service   = 2
)

// Map of format string :: expected locations of name symbols in the format.
// -1 value indicates that symbol does not exist in format.
var exampleTemplates = map[string][]int{
	"{service}.{namespace}.{type}.{zone}": []int{3, 1, 0}, // service symbol expected @ position 0, namespace @ 1, zone @ 3
	"{namespace}.{type}.{zone}":           []int{2, 0, -1},
	"":                             []int{-1, -1, -1},
}

func TestSetTemplate(t *testing.T) {
	for s, expectedValue := range exampleTemplates {

		n := new(Template)
		n.SetTemplate(s)

		// check the indexes resulting from calling SetTemplate() against expectedValues
		if expectedValue[zone] != -1 {
			if n.Element["zone"] != expectedValue[zone] {
				t.Errorf("Expected zone at index '%v', instead found at index '%v' for format string '%v'", expectedValue[zone], n.Element["zone"], s)
			}
		}
	}
}

func TestServiceFromSegmentArray(t *testing.T) {
	var (
		n               *Template
		formatString    string
		queryString     string
		splitQuery      []string
		expectedService string
		actualService   string
	)

	// Case where template contains {service}
	n = new(Template)
	formatString = "{service}.{namespace}.{type}.{zone}"
	n.SetTemplate(formatString)

	queryString = "myservice.mynamespace.svc.coredns"
	splitQuery = strings.Split(queryString, ".")
	expectedService = "myservice"
	actualService = n.ServiceFromSegmentArray(splitQuery)

	if actualService != expectedService {
		t.Errorf("Expected service name '%v', instead got service name '%v' for query string '%v' and format '%v'", expectedService, actualService, queryString, formatString)
	}

	// Case where template does not contain {service}
	n = new(Template)
	formatString = "{namespace}.{type}.{zone}"
	n.SetTemplate(formatString)

	queryString = "mynamespace.svc.coredns"
	splitQuery = strings.Split(queryString, ".")
	expectedService = ""
	actualService = n.ServiceFromSegmentArray(splitQuery)

	if actualService != expectedService {
		t.Errorf("Expected service name '%v', instead got service name '%v' for query string '%v' and format '%v'", expectedService, actualService, queryString, formatString)
	}
}

func TestZoneFromSegmentArray(t *testing.T) {
	var (
		n            *Template
		formatString string
		queryString  string
		splitQuery   []string
		expectedZone string
		actualZone   string
	)

	// Case where template contains {zone}
	n = new(Template)
	formatString = "{service}.{namespace}.{type}.{zone}"
	n.SetTemplate(formatString)

	queryString = "myservice.mynamespace.svc.coredns"
	splitQuery = strings.Split(queryString, ".")
	expectedZone = "coredns"
	actualZone = n.ZoneFromSegmentArray(splitQuery)

	if actualZone != expectedZone {
		t.Errorf("Expected zone name '%v', instead got zone name '%v' for query string '%v' and format '%v'", expectedZone, actualZone, queryString, formatString)
	}

	// Case where template does not contain {zone}
	n = new(Template)
	formatString = "{service}.{namespace}.{type}"
	n.SetTemplate(formatString)

	queryString = "mynamespace.coredns.svc"
	splitQuery = strings.Split(queryString, ".")
	expectedZone = ""
	actualZone = n.ZoneFromSegmentArray(splitQuery)

	if actualZone != expectedZone {
		t.Errorf("Expected zone name '%v', instead got zone name '%v' for query string '%v' and format '%v'", expectedZone, actualZone, queryString, formatString)
	}

	// Case where zone is multiple segments
	n = new(Template)
	formatString = "{service}.{namespace}.{type}.{zone}"
	n.SetTemplate(formatString)

	queryString = "myservice.mynamespace.svc.coredns.cluster.local"
	splitQuery = strings.Split(queryString, ".")
	expectedZone = "coredns.cluster.local"
	actualZone = n.ZoneFromSegmentArray(splitQuery)

	if actualZone != expectedZone {
		t.Errorf("Expected zone name '%v', instead got zone name '%v' for query string '%v' and format '%v'", expectedZone, actualZone, queryString, formatString)
	}
}
