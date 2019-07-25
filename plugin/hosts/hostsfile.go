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

// parseIP calls discards any v6 zone info, before calling net.ParseIP.
func parseIP(addr string) net.IP {
	if i := strings.Index(addr, "%"); i >= 0 {
		// discard ipv6 zone
		addr = addr[0:i]
	}

	return net.ParseIP(addr)
}

type options struct {
	// automatically generate IP to Hostname PTR entries
	// for host entries we parse
	autoReverse bool

	// The TTL of the record we generate
	ttl uint32

	// The time between two reload of the configuration
	reload time.Duration
}

func newOptions() *options {
	return &options{
		autoReverse: true,
		ttl:         3600,
		reload:      time.Duration(5 * time.Second),
	}
}

// Map contains the IPv4/IPv6 and reverse mapping.
type Map struct {
	// Key for the list of literal IP addresses must be a FQDN lowercased host name.
	name4 map[string][]net.IP
	name6 map[string][]net.IP

	// Key for the list of host names must be a literal IP address
	// including IPv6 address without zone identifier.
	// We don't support old-classful IP address notation.
	addr map[string][]string
}

func newMap() *Map {
	return &Map{
		name4: make(map[string][]net.IP),
		name6: make(map[string][]net.IP),
		addr:  make(map[string][]string),
	}
}

// Len returns the total number of addresses in the hostmap, this includes V4/V6 and any reverse addresses.
func (h *Map) Len() int {
	l := 0
	for _, v4 := range h.name4 {
		l += len(v4)
	}
	for _, v6 := range h.name6 {
		l += len(v6)
	}
	for _, a := range h.addr {
		l += len(a)
	}
	return l
}

// Hostsfile contains known host entries.
type Hostsfile struct {
	sync.RWMutex

	// list of zones we are authoritative for
	Origins []string

	// hosts maps for lookups
	hmap *Map

	// inline saves the hosts file that is inlined in a Corefile.
	inline *Map

	// path to the hosts file
	path string

	// mtime and size are only read and modified by a single goroutine
	mtime time.Time
	size  int64

	options *options
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
	h.RLock()
	size := h.size
	h.RUnlock()

	if err == nil && h.mtime.Equal(stat.ModTime()) && size == stat.Size() {
		return
	}

	newMap := h.parse(file)
	log.Debugf("Parsed hosts file into %d entries", newMap.Len())

	h.Lock()

	h.hmap = newMap
	// Update the data cache.
	h.mtime = stat.ModTime()
	h.size = stat.Size()

	h.Unlock()
}

func (h *Hostsfile) initInline(inline []string) {
	if len(inline) == 0 {
		return
	}

	h.inline = h.parse(strings.NewReader(strings.Join(inline, "\n")))
}

// Parse reads the hostsfile and populates the byName and addr maps.
func (h *Hostsfile) parse(r io.Reader) *Map {
	hmap := newMap()

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
		addr := parseIP(string(f[0]))
		if addr == nil {
			continue
		}

		family := 0
		if addr.To4() != nil {
			family = 1
		} else {
			family = 2
		}

		for i := 1; i < len(f); i++ {
			name := plugin.Name(string(f[i])).Normalize()
			if plugin.Zones(h.Origins).Matches(name) == "" {
				// name is not in Origins
				continue
			}
			switch family {
			case 1:
				hmap.name4[name] = append(hmap.name4[name], addr)
			case 2:
				hmap.name6[name] = append(hmap.name6[name], addr)
			default:
				continue
			}
			if !h.options.autoReverse {
				continue
			}
			hmap.addr[addr.String()] = append(hmap.addr[addr.String()], name)
		}
	}

	return hmap
}

// lookupStaticHost looks up the IP addresses for the given host from the hosts file.
func (h *Hostsfile) lookupStaticHost(m map[string][]net.IP, host string) []net.IP {
	h.RLock()
	defer h.RUnlock()

	if len(m) == 0 {
		return nil
	}

	ips, ok := m[host]
	if !ok {
		return nil
	}
	ipsCp := make([]net.IP, len(ips))
	copy(ipsCp, ips)
	return ipsCp
}

// LookupStaticHostV4 looks up the IPv4 addresses for the given host from the hosts file.
func (h *Hostsfile) LookupStaticHostV4(host string) []net.IP {
	host = strings.ToLower(host)
	ip1 := h.lookupStaticHost(h.hmap.name4, host)
	ip2 := h.lookupStaticHost(h.inline.name4, host)
	return append(ip1, ip2...)
}

// LookupStaticHostV6 looks up the IPv6 addresses for the given host from the hosts file.
func (h *Hostsfile) LookupStaticHostV6(host string) []net.IP {
	host = strings.ToLower(host)
	ip1 := h.lookupStaticHost(h.hmap.name6, host)
	ip2 := h.lookupStaticHost(h.inline.name6, host)
	return append(ip1, ip2...)
}

// LookupStaticAddr looks up the hosts for the given address from the hosts file.
func (h *Hostsfile) LookupStaticAddr(addr string) []string {
	addr = parseIP(addr).String()
	if addr == "" {
		return nil
	}

	h.RLock()
	defer h.RUnlock()
	hosts1, _ := h.hmap.addr[addr]
	hosts2, _ := h.inline.addr[addr]

	if len(hosts1) == 0 && len(hosts2) == 0 {
		return nil
	}

	hostsCp := make([]string, len(hosts1)+len(hosts2))
	copy(hostsCp, hosts1)
	copy(hostsCp[len(hosts1):], hosts2)
	return hostsCp
}
