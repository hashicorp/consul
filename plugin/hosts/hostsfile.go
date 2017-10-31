// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is a modified version of net/hosts.go from the golang repo

package hosts

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
)

func parseLiteralIP(addr string) net.IP {
	if i := strings.Index(addr, "%"); i >= 0 {
		// discard ipv6 zone
		addr = addr[0:i]
	}

	return net.ParseIP(addr)
}

func absDomainName(b string) string {
	return plugin.Name(b).Normalize()
}

type hostsMap struct {
	// Key for the list of literal IP addresses must be a host
	// name. It would be part of DNS labels, a FQDN or an absolute
	// FQDN.
	// For now the key is converted to lower case for convenience.
	byNameV4 map[string][]net.IP
	byNameV6 map[string][]net.IP

	// Key for the list of host names must be a literal IP address
	// including IPv6 address with zone identifier.
	// We don't support old-classful IP address notation.
	byAddr map[string][]string
}

func newHostsMap() *hostsMap {
	return &hostsMap{
		byNameV4: make(map[string][]net.IP),
		byNameV6: make(map[string][]net.IP),
		byAddr:   make(map[string][]string),
	}
}

// Hostsfile contains known host entries.
type Hostsfile struct {
	sync.RWMutex

	// list of zones we are authoritive for
	Origins []string

	// hosts maps for lookups
	hmap *hostsMap

	// inline saves the hosts file that is inlined in a Corefile.
	// We need a copy here as we want to use it to initialize the maps for parse.
	inline *hostsMap

	// path to the hosts file
	path string

	// mtime and size are only read and modified by a single goroutine
	mtime time.Time
	size  int64
}

// readHosts determines if the cached data needs to be updated based on the size and modification time of the hostsfile.
func (h *Hostsfile) readHosts() {
	file, err := os.Open(h.path)
	if err != nil {
		// We already log a warning if the file doesn't exist or can't be opened on setup. No need to return the error here.
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err == nil && h.mtime.Equal(stat.ModTime()) && h.size == stat.Size() {
		return
	}

	h.Lock()
	defer h.Unlock()
	h.parseReader(file)

	// Update the data cache.
	h.mtime = stat.ModTime()
	h.size = stat.Size()
}

func (h *Hostsfile) initInline(inline []string) {
	if len(inline) == 0 {
		return
	}

	hmap := newHostsMap()
	h.inline = h.parse(strings.NewReader(strings.Join(inline, "\n")), hmap)
	*h.hmap = *h.inline
}

func (h *Hostsfile) parseReader(r io.Reader) {
	h.hmap = h.parse(r, h.inline)
}

// Parse reads the hostsfile and populates the byName and byAddr maps.
func (h *Hostsfile) parse(r io.Reader, override *hostsMap) *hostsMap {
	hmap := newHostsMap()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if i := bytes.Index(line, []byte{'#'}); i >= 0 {
			// Discard comments.
			line = line[0:i]
		}
		f := bytes.Fields(line)
		if len(f) < 2 {
			continue
		}
		addr := parseLiteralIP(string(f[0]))
		if addr == nil {
			continue
		}
		ver := ipVersion(string(f[0]))
		for i := 1; i < len(f); i++ {
			name := absDomainName(string(f[i]))
			if plugin.Zones(h.Origins).Matches(name) == "" {
				// name is not in Origins
				continue
			}
			switch ver {
			case 4:
				hmap.byNameV4[name] = append(hmap.byNameV4[name], addr)
			case 6:
				hmap.byNameV6[name] = append(hmap.byNameV6[name], addr)
			default:
				continue
			}
			hmap.byAddr[addr.String()] = append(hmap.byAddr[addr.String()], name)
		}
	}

	if override == nil {
		return hmap
	}

	for name := range override.byNameV4 {
		hmap.byNameV4[name] = append(hmap.byNameV4[name], override.byNameV4[name]...)
	}
	for name := range override.byNameV4 {
		hmap.byNameV6[name] = append(hmap.byNameV6[name], override.byNameV6[name]...)
	}
	for addr := range override.byAddr {
		hmap.byAddr[addr] = append(hmap.byAddr[addr], override.byAddr[addr]...)
	}

	return hmap
}

// ipVersion returns what IP version was used textually
func ipVersion(s string) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return 4
		case ':':
			return 6
		}
	}
	return 0
}

// LookupStaticHostV4 looks up the IPv4 addresses for the given host from the hosts file.
func (h *Hostsfile) LookupStaticHostV4(host string) []net.IP {
	h.RLock()
	defer h.RUnlock()
	if len(h.hmap.byNameV4) != 0 {
		if ips, ok := h.hmap.byNameV4[absDomainName(host)]; ok {
			ipsCp := make([]net.IP, len(ips))
			copy(ipsCp, ips)
			return ipsCp
		}
	}
	return nil
}

// LookupStaticHostV6 looks up the IPv6 addresses for the given host from the hosts file.
func (h *Hostsfile) LookupStaticHostV6(host string) []net.IP {
	h.RLock()
	defer h.RUnlock()
	if len(h.hmap.byNameV6) != 0 {
		if ips, ok := h.hmap.byNameV6[absDomainName(host)]; ok {
			ipsCp := make([]net.IP, len(ips))
			copy(ipsCp, ips)
			return ipsCp
		}
	}
	return nil
}

// LookupStaticAddr looks up the hosts for the given address from the hosts file.
func (h *Hostsfile) LookupStaticAddr(addr string) []string {
	h.RLock()
	defer h.RUnlock()
	addr = parseLiteralIP(addr).String()
	if addr == "" {
		return nil
	}
	if len(h.hmap.byAddr) != 0 {
		if hosts, ok := h.hmap.byAddr[addr]; ok {
			hostsCp := make([]string, len(hosts))
			copy(hostsCp, hosts)
			return hostsCp
		}
	}
	return nil
}
