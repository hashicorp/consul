// Package secondary implements a secondary middleware.
package secondary

import "github.com/miekg/coredns/middleware/file"

// Secondary implements a secondary middleware that allows CoreDNS to retrieve (via AXFR)
// zone information from a primary server.
type Secondary struct {
	file.File
}
