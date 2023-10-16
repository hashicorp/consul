// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/structs"
)

type TranslateAddressAccept int

const (
	TranslateAddressAcceptDomain TranslateAddressAccept = 1 << iota
	TranslateAddressAcceptIPv4
	TranslateAddressAcceptIPv6

	TranslateAddressAcceptAny TranslateAddressAccept = ^0
)

// TranslateServicePort is used to provide the final, translated port for a service,
// depending on how the agent and the other node are configured. The dc
// parameter is the dc the datacenter this node is from.
func (a *Agent) TranslateServicePort(dc string, port int, taggedAddresses map[string]structs.ServiceAddress) int {
	if a.config.TranslateWANAddrs && (a.config.Datacenter != dc) {
		if wanAddr, ok := taggedAddresses[structs.TaggedAddressWAN]; ok && wanAddr.Port != 0 {
			return wanAddr.Port
		}
	}
	return port
}

// TranslateServiceAddress is used to provide the final, translated address for a node,
// depending on how the agent and the other node are configured. The dc
// parameter is the dc the datacenter this node is from.
func (a *Agent) TranslateServiceAddress(dc string, addr string, taggedAddresses map[string]structs.ServiceAddress, accept TranslateAddressAccept) string {
	def := addr
	v4 := taggedAddresses[structs.TaggedAddressLANIPv4].Address
	v6 := taggedAddresses[structs.TaggedAddressLANIPv6].Address

	shouldUseWan := a.config.TranslateWANAddrs && (a.config.Datacenter != dc)
	if shouldUseWan {
		if v, ok := taggedAddresses[structs.TaggedAddressWAN]; ok {
			def = v.Address
		}
		if v, ok := taggedAddresses[structs.TaggedAddressWANIPv4]; ok {
			v4 = v.Address
		}
		if v, ok := taggedAddresses[structs.TaggedAddressWANIPv6]; ok {
			v6 = v.Address
		}
	}

	return translateAddressAccept(accept, def, v4, v6)
}

// TranslateAddress is used to provide the final, translated address for a node,
// depending on how the agent and the other node are configured. The dc
// parameter is the dc the datacenter this node is from.
func (a *Agent) TranslateAddress(dc string, addr string, taggedAddresses map[string]string, accept TranslateAddressAccept) string {
	def := addr
	v4 := taggedAddresses[structs.TaggedAddressLANIPv4]
	v6 := taggedAddresses[structs.TaggedAddressLANIPv6]

	shouldUseWan := a.config.TranslateWANAddrs && (a.config.Datacenter != dc)
	if shouldUseWan {
		if v, ok := taggedAddresses[structs.TaggedAddressWAN]; ok {
			def = v
		}
		if v, ok := taggedAddresses[structs.TaggedAddressWANIPv4]; ok {
			v4 = v
		}
		if v, ok := taggedAddresses[structs.TaggedAddressWANIPv6]; ok {
			v6 = v
		}
	}

	return translateAddressAccept(accept, def, v4, v6)
}

func translateAddressAccept(accept TranslateAddressAccept, def, v4, v6 string) string {
	switch {
	case accept&TranslateAddressAcceptIPv6 > 0 && v6 != "":
		return v6
	case accept&TranslateAddressAcceptIPv4 > 0 && v4 != "":
		return v4
	case accept&TranslateAddressAcceptAny > 0 && def != "":
		return def
	default:
		defIP := net.ParseIP(def)
		switch {
		case defIP != nil && defIP.To4() != nil && accept&TranslateAddressAcceptIPv4 > 0:
			return def
		case defIP != nil && defIP.To4() == nil && accept&TranslateAddressAcceptIPv6 > 0:
			return def
		case defIP == nil && accept&TranslateAddressAcceptDomain > 0:
			return def
		}
	}

	return ""
}

// TranslateAddresses translates addresses in the given structure into the
// final, translated address, depending on how the agent and the other node are
// configured. The dc parameter is the datacenter this structure is from.
func (a *Agent) TranslateAddresses(dc string, subj interface{}, accept TranslateAddressAccept) {
	// CAUTION - SUBTLE! An agent running on a server can, in some cases,
	// return pointers directly into the immutable state store for
	// performance (it's via the in-memory RPC mechanism). It's never safe
	// to modify those values, so we short circuit here so that we never
	// update any structures that are from our own datacenter. This works
	// for address translation because we *never* need to translate local
	// addresses, but this is super subtle, so we've piped all the in-place
	// address translation into this function which makes sure this check is
	// done. This also happens to skip looking at any of the incoming
	// structure for the common case of not needing to translate, so it will
	// skip a lot of work if no translation needs to be done.
	if !a.config.TranslateWANAddrs || (a.config.Datacenter == dc) {
		return
	}

	// Translate addresses in-place, subject to the condition checked above
	// which ensures this is safe to do since we are operating on a local
	// copy of the data.
	switch v := subj.(type) {
	case structs.CheckServiceNodes:
		for _, entry := range v {
			entry.Node.Address = a.TranslateAddress(dc, entry.Node.Address, entry.Node.TaggedAddresses, accept)
			entry.Service.Address = a.TranslateServiceAddress(dc, entry.Service.Address, entry.Service.TaggedAddresses, accept)
			entry.Service.Port = a.TranslateServicePort(dc, entry.Service.Port, entry.Service.TaggedAddresses)
		}
	case *structs.Node:
		v.Address = a.TranslateAddress(dc, v.Address, v.TaggedAddresses, accept)
	case structs.Nodes:
		for _, node := range v {
			node.Address = a.TranslateAddress(dc, node.Address, node.TaggedAddresses, accept)
		}
	case structs.ServiceNodes:
		for _, entry := range v {
			entry.Address = a.TranslateAddress(dc, entry.Address, entry.TaggedAddresses, accept)
			entry.ServiceAddress = a.TranslateServiceAddress(dc, entry.ServiceAddress, entry.ServiceTaggedAddresses, accept)
			entry.ServicePort = a.TranslateServicePort(dc, entry.ServicePort, entry.ServiceTaggedAddresses)
		}
	case *structs.NodeServices:
		if v.Node != nil {
			v.Node.Address = a.TranslateAddress(dc, v.Node.Address, v.Node.TaggedAddresses, accept)
		}
		for _, entry := range v.Services {
			entry.Address = a.TranslateServiceAddress(dc, entry.Address, entry.TaggedAddresses, accept)
			entry.Port = a.TranslateServicePort(dc, entry.Port, entry.TaggedAddresses)
		}
	case *structs.NodeServiceList:
		if v.Node != nil {
			v.Node.Address = a.TranslateAddress(dc, v.Node.Address, v.Node.TaggedAddresses, accept)
		}
		for _, entry := range v.Services {
			entry.Address = a.TranslateServiceAddress(dc, entry.Address, entry.TaggedAddresses, accept)
			entry.Port = a.TranslateServicePort(dc, entry.Port, entry.TaggedAddresses)
		}
	default:
		panic(fmt.Errorf("Unhandled type passed to address translator: %#v", subj))
	}
}
