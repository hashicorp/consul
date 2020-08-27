package dns

import "regexp"

// MaxLabelLength is the maximum length for a name that can be used in DNS.
const MaxLabelLength = 63

// InvalidNameRe is a regex that matches characters which can not be included in
// a DNS name.
var InvalidNameRe = regexp.MustCompile(`[^A-Za-z0-9\\-]+`)
