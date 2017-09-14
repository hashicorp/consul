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

const cacheMaxAge = 5 * time.Second

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

// Hostsfile contains known host entries.
type Hostsfile struct {
	sync.Mutex

	// list of zones we are authoritive for
	Origins []string

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

	expire time.Time
	path   string
	mtime  time.Time
	size   int64
}

// ReadHosts determines if the cached data needs to be updated based on the size and modification time of the hostsfile.
func (h *Hostsfile) ReadHosts() {
	now := time.Now()

	if now.Before(h.expire) && len(h.byAddr) > 0 {
		return
	}
	stat, err := os.Stat(h.path)
	if err == nil && h.mtime.Equal(stat.ModTime()) && h.size == stat.Size() {
		h.expire = now.Add(cacheMaxAge)
		return
	}

	var file *os.File
	if file, _ = os.Open(h.path); file == nil {
		return
	}
	defer file.Close()

	h.Parse(file)

	// Update the data cache.
	h.expire = now.Add(cacheMaxAge)
	h.mtime = stat.ModTime()
	h.size = stat.Size()
}

// Parse reads the hostsfile and populates the byName and byAddr maps.
func (h *Hostsfile) Parse(file io.Reader) {
	hsv4 := make(map[string][]net.IP)
	hsv6 := make(map[string][]net.IP)
	is := make(map[string][]string)

	scanner := bufio.NewScanner(file)
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
				hsv4[name] = append(hsv4[name], addr)
			case 6:
				hsv6[name] = append(hsv6[name], addr)
			default:
				continue
			}
			is[addr.String()] = append(is[addr.String()], name)
		}
	}
	h.byNameV4 = hsv4
	h.byNameV6 = hsv6
	h.byAddr = is
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
	h.Lock()
	defer h.Unlock()
	h.ReadHosts()
	if len(h.byNameV4) != 0 {
		if ips, ok := h.byNameV4[absDomainName(host)]; ok {
			ipsCp := make([]net.IP, len(ips))
			copy(ipsCp, ips)
			return ipsCp
		}
	}
	return nil
}

// LookupStaticHostV6 looks up the IPv6 addresses for the given host from the hosts file.
func (h *Hostsfile) LookupStaticHostV6(host string) []net.IP {
	h.Lock()
	defer h.Unlock()
	h.ReadHosts()
	if len(h.byNameV6) != 0 {
		if ips, ok := h.byNameV6[absDomainName(host)]; ok {
			ipsCp := make([]net.IP, len(ips))
			copy(ipsCp, ips)
			return ipsCp
		}
	}
	return nil
}

// LookupStaticAddr looks up the hosts for the given address from the hosts file.
func (h *Hostsfile) LookupStaticAddr(addr string) []string {
	h.Lock()
	defer h.Unlock()
	h.ReadHosts()
	addr = parseLiteralIP(addr).String()
	if addr == "" {
		return nil
	}
	if len(h.byAddr) != 0 {
		if hosts, ok := h.byAddr[addr]; ok {
			hostsCp := make([]string, len(hosts))
			copy(hostsCp, hosts)
			return hostsCp
		}
	}
	return nil
}
