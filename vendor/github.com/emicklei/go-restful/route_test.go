package restful

import (
	"testing"
)

// accept should match produces
func TestMatchesAcceptPlainTextWhenProducePlainTextAsLast(t *testing.T) {
	r := Route{Produces: []string{"application/json", "text/plain"}}
	if !r.matchesAccept("text/plain") {
		t.Errorf("accept should match text/plain")
	}
}

// accept should match produces
func TestMatchesAcceptStar(t *testing.T) {
	r := Route{Produces: []string{"application/xml"}}
	if !r.matchesAccept("*/*") {
		t.Errorf("accept should match star")
	}
}

// accept should match produces
func TestMatchesAcceptIE(t *testing.T) {
	r := Route{Produces: []string{"application/xml"}}
	if !r.matchesAccept("text/html, application/xhtml+xml, */*") {
		t.Errorf("accept should match star")
	}
}

// accept should match produces
func TestMatchesAcceptXml(t *testing.T) {
	r := Route{Produces: []string{"application/xml"}}
	if r.matchesAccept("application/json") {
		t.Errorf("accept should not match json")
	}
	if !r.matchesAccept("application/xml") {
		t.Errorf("accept should match xml")
	}
}

// accept should match produces
func TestMatchesAcceptAny(t *testing.T) {
	r := Route{Produces: []string{"*/*"}}
	if !r.matchesAccept("application/json") {
		t.Errorf("accept should match json")
	}
	if !r.matchesAccept("application/xml") {
		t.Errorf("accept should match xml")
	}
}

// content type should match consumes
func TestMatchesContentTypeXml(t *testing.T) {
	r := Route{Consumes: []string{"application/xml"}}
	if r.matchesContentType("application/json") {
		t.Errorf("accept should not match json")
	}
	if !r.matchesContentType("application/xml") {
		t.Errorf("accept should match xml")
	}
}

// content type should match consumes
func TestMatchesContentTypeCharsetInformation(t *testing.T) {
	r := Route{Consumes: []string{"application/json"}}
	if !r.matchesContentType("application/json; charset=UTF-8") {
		t.Errorf("matchesContentType should ignore charset information")
	}
}

func TestTokenizePath(t *testing.T) {
	if len(tokenizePath("/")) != 0 {
		t.Errorf("not empty path tokens")
	}
}
