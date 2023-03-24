package connect

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"net"
	"net/url"
	"unicode"
)

// NOTE: the contents of this file were lifted from
// $GOROOT/src/crypto/x509/x509.go from a Go 1.16.5 checkout.
//
//
// After https://go-review.googlesource.com/c/go/+/329129 lands in a Go release
// we are compiling against we can safely remove all of this code.

var (
	x509_oidExtensionSubjectAltName = []int{2, 5, 29, 17}
)

const (
	x509_nameTypeEmail = 1
	x509_nameTypeDNS   = 2
	x509_nameTypeURI   = 6
	x509_nameTypeIP    = 7
)

// HackSANExtensionForCSR will create a SAN extension on the CSR off of the
// convenience fields (DNSNames, EmailAddresses, IPAddresses, URIs) and
// appropriately marks that SAN extension as critical if the CSR has an empty
// subject.
//
// This is basically attempting to repeat this blob of code from the stdlib
// ourselves:
//
// https://github.com/golang/go/blob/0e67ce3d28320e816dd8e7cf7d701c1804fb977e/src/crypto/x509/x509.go#L1088
func HackSANExtensionForCSR(template *x509.CertificateRequest) {
	switch {
	case len(template.DNSNames) > 0:
	case len(template.EmailAddresses) > 0:
	case len(template.IPAddresses) > 0:
	case len(template.URIs) > 0:
	default:
		return
	}

	if x509_oidInExtensions(x509_oidExtensionSubjectAltName, template.ExtraExtensions) {
		return
	}

	value, err := x509_marshalSANs(template.DNSNames, template.EmailAddresses, template.IPAddresses, template.URIs)
	if err != nil {
		return
	}

	ext := pkix.Extension{
		Id: x509_oidExtensionSubjectAltName,
		// From RFC 5280, Section 4.2.1.6:
		// “If the subject field contains an empty sequence ... then
		// subjectAltName extension ... is marked as critical”
		//
		// Since we just cleared the subject above, it's critical.
		Critical: true,
		Value:    value,
	}
	template.ExtraExtensions = append(template.ExtraExtensions, ext)
}

// x509_oidInExtensions reports whether an extension with the given oid exists in
// extensions.
func x509_oidInExtensions(oid asn1.ObjectIdentifier, extensions []pkix.Extension) bool {
	for _, e := range extensions {
		if e.Id.Equal(oid) {
			return true
		}
	}
	return false
}

// x509_marshalSANs marshals a list of addresses into a the contents of an X.509
// SubjectAlternativeName extension.
func x509_marshalSANs(dnsNames, emailAddresses []string, ipAddresses []net.IP, uris []*url.URL) (derBytes []byte, err error) {
	var rawValues []asn1.RawValue
	for _, name := range dnsNames {
		if err := x509_isIA5String(name); err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: x509_nameTypeDNS, Class: 2, Bytes: []byte(name)})
	}
	for _, email := range emailAddresses {
		if err := x509_isIA5String(email); err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: x509_nameTypeEmail, Class: 2, Bytes: []byte(email)})
	}
	for _, rawIP := range ipAddresses {
		// If possible, we always want to encode IPv4 addresses in 4 bytes.
		ip := rawIP.To4()
		if ip == nil {
			ip = rawIP
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: x509_nameTypeIP, Class: 2, Bytes: ip})
	}
	for _, uri := range uris {
		uriStr := uri.String()
		if err := x509_isIA5String(uriStr); err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: x509_nameTypeURI, Class: 2, Bytes: []byte(uriStr)})
	}
	return asn1.Marshal(rawValues)
}

func x509_isIA5String(s string) error {
	for _, r := range s {
		// Per RFC5280 "IA5String is limited to the set of ASCII characters"
		if r > unicode.MaxASCII {
			return fmt.Errorf("x509: %q cannot be encoded as an IA5String", s)
		}
	}

	return nil
}
