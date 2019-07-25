// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hosts

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin"
)

func testHostsfile(file string) *Hostsfile {
	h := &Hostsfile{
		Origins: []string{"."},
		hmap:    newMap(),
		inline:  newMap(),
		options: newOptions(),
	}
	h.hmap = h.parse(strings.NewReader(file))
	return h
}

type staticHostEntry struct {
	in string
	v4 []string
	v6 []string
}

var (
	hosts = `255.255.255.255	broadcasthost
	127.0.0.2	odin
	127.0.0.3	odin  # inline comment
	::2             odin
	127.1.1.1	thor
	# aliases
	127.1.1.2	ullr ullrhost
	fe80::1%lo0	localhost
	# Bogus entries that must be ignored.
	123.123.123	loki
	321.321.321.321`
	singlelinehosts = `127.0.0.2  odin`
	ipv4hosts       = `# See https://tools.ietf.org/html/rfc1123.
	#
	# The literal IPv4 address parser in the net package is a relaxed
	# one. It may accept a literal IPv4 address in dotted-decimal notation
	# with leading zeros such as "001.2.003.4".

	# internet address and host name
	127.0.0.1	localhost	# inline comment separated by tab
	127.000.000.002	localhost       # inline comment separated by space

	# internet address, host name and aliases
	127.000.000.003	localhost	localhost.localdomain`
	ipv6hosts = `# See https://tools.ietf.org/html/rfc5952, https://tools.ietf.org/html/rfc4007.

	# internet address and host name
	::1						localhost	# inline comment separated by tab
	fe80:0000:0000:0000:0000:0000:0000:0001		localhost       # inline comment separated by space

	# internet address with zone identifier and host name
	fe80:0000:0000:0000:0000:0000:0000:0002%lo0	localhost

	# internet address, host name and aliases
	fe80::3%lo0					localhost	localhost.localdomain`
	casehosts = `127.0.0.1	PreserveMe	PreserveMe.local
		::1		PreserveMe	PreserveMe.local`
)

var lookupStaticHostTests = []struct {
	file string
	ents []staticHostEntry
}{
	{
		hosts,
		[]staticHostEntry{
			{"odin.", []string{"127.0.0.2", "127.0.0.3"}, []string{"::2"}},
			{"thor.", []string{"127.1.1.1"}, []string{}},
			{"ullr.", []string{"127.1.1.2"}, []string{}},
			{"ullrhost.", []string{"127.1.1.2"}, []string{}},
			{"localhost.", []string{}, []string{"fe80::1"}},
		},
	},
	{
		singlelinehosts, // see golang.org/issue/6646
		[]staticHostEntry{
			{"odin.", []string{"127.0.0.2"}, []string{}},
		},
	},
	{
		ipv4hosts,
		[]staticHostEntry{
			{"localhost.", []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, []string{}},
			{"localhost.localdomain.", []string{"127.0.0.3"}, []string{}},
		},
	},
	{
		ipv6hosts,
		[]staticHostEntry{
			{"localhost.", []string{}, []string{"::1", "fe80::1", "fe80::2", "fe80::3"}},
			{"localhost.localdomain.", []string{}, []string{"fe80::3"}},
		},
	},
	{
		casehosts,
		[]staticHostEntry{
			{"PreserveMe.", []string{"127.0.0.1"}, []string{"::1"}},
			{"PreserveMe.local.", []string{"127.0.0.1"}, []string{"::1"}},
		},
	},
}

func TestLookupStaticHost(t *testing.T) {
	for _, tt := range lookupStaticHostTests {
		h := testHostsfile(tt.file)
		for _, ent := range tt.ents {
			testStaticHost(t, ent, h)
		}
	}
}

func testStaticHost(t *testing.T, ent staticHostEntry, h *Hostsfile) {
	ins := []string{ent.in, plugin.Name(ent.in).Normalize(), strings.ToLower(ent.in), strings.ToUpper(ent.in)}
	for k, in := range ins {
		addrsV4 := h.LookupStaticHostV4(in)
		if len(addrsV4) != len(ent.v4) {
			t.Fatalf("%d, lookupStaticHostV4(%s) = %v; want %v", k, in, addrsV4, ent.v4)
		}
		for i, v4 := range addrsV4 {
			if v4.String() != ent.v4[i] {
				t.Fatalf("%d, lookupStaticHostV4(%s) = %v; want %v", k, in, addrsV4, ent.v4)
			}
		}
		addrsV6 := h.LookupStaticHostV6(in)
		if len(addrsV6) != len(ent.v6) {
			t.Fatalf("%d, lookupStaticHostV6(%s) = %v; want %v", k, in, addrsV6, ent.v6)
		}
		for i, v6 := range addrsV6 {
			if v6.String() != ent.v6[i] {
				t.Fatalf("%d, lookupStaticHostV6(%s) = %v; want %v", k, in, addrsV6, ent.v6)
			}
		}
	}
}

type staticIPEntry struct {
	in  string
	out []string
}

var lookupStaticAddrTests = []struct {
	file string
	ents []staticIPEntry
}{
	{
		hosts,
		[]staticIPEntry{
			{"255.255.255.255", []string{"broadcasthost."}},
			{"127.0.0.2", []string{"odin."}},
			{"127.0.0.3", []string{"odin."}},
			{"::2", []string{"odin."}},
			{"127.1.1.1", []string{"thor."}},
			{"127.1.1.2", []string{"ullr.", "ullrhost."}},
			{"fe80::1", []string{"localhost."}},
		},
	},
	{
		singlelinehosts, // see golang.org/issue/6646
		[]staticIPEntry{
			{"127.0.0.2", []string{"odin."}},
		},
	},
	{
		ipv4hosts, // see golang.org/issue/8996
		[]staticIPEntry{
			{"127.0.0.1", []string{"localhost."}},
			{"127.0.0.2", []string{"localhost."}},
			{"127.0.0.3", []string{"localhost.", "localhost.localdomain."}},
		},
	},
	{
		ipv6hosts, // see golang.org/issue/8996
		[]staticIPEntry{
			{"::1", []string{"localhost."}},
			{"fe80::1", []string{"localhost."}},
			{"fe80::2", []string{"localhost."}},
			{"fe80::3", []string{"localhost.", "localhost.localdomain."}},
		},
	},
	{
		casehosts, // see golang.org/issue/12806
		[]staticIPEntry{
			{"127.0.0.1", []string{"PreserveMe.", "PreserveMe.local."}},
			{"::1", []string{"PreserveMe.", "PreserveMe.local."}},
		},
	},
}

func TestLookupStaticAddr(t *testing.T) {
	for _, tt := range lookupStaticAddrTests {
		h := testHostsfile(tt.file)
		for _, ent := range tt.ents {
			testStaticAddr(t, ent, h)
		}
	}
}

func testStaticAddr(t *testing.T, ent staticIPEntry, h *Hostsfile) {
	hosts := h.LookupStaticAddr(ent.in)
	for i := range ent.out {
		ent.out[i] = plugin.Name(ent.out[i]).Normalize()
	}
	if !reflect.DeepEqual(hosts, ent.out) {
		t.Errorf("%s, lookupStaticAddr(%s) = %v; want %v", h.path, ent.in, hosts, h)
	}
}

func TestHostCacheModification(t *testing.T) {
	// Ensure that programs can't modify the internals of the host cache.
	// See https://github.com/golang/go/issues/14212.

	h := testHostsfile(ipv4hosts)
	ent := staticHostEntry{"localhost.", []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, []string{}}
	testStaticHost(t, ent, h)
	// Modify the addresses return by lookupStaticHost.
	addrs := h.LookupStaticHostV6(ent.in)
	for i := range addrs {
		addrs[i] = net.IPv4zero
	}
	testStaticHost(t, ent, h)

	h = testHostsfile(ipv6hosts)
	entip := staticIPEntry{"::1", []string{"localhost."}}
	testStaticAddr(t, entip, h)
	// Modify the hosts return by lookupStaticAddr.
	hosts := h.LookupStaticAddr(entip.in)
	for i := range hosts {
		hosts[i] += "junk"
	}
	testStaticAddr(t, entip, h)
}
