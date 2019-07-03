package agent

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

// TranslateServicePort is used to provide the final, translated port for a service,
// depending on how the agent and the other node are configured. The dc
// parameter is the dc the datacenter this node is from.
func (a *Agent) TranslateServicePort(dc string, port int, taggedAddresses map[string]structs.ServiceAddress) int {
	if a.config.TranslateWANAddrs && (a.config.Datacenter != dc) {
		if wanAddr, ok := taggedAddresses["wan"]; ok && wanAddr.Port != 0 {
			return wanAddr.Port
		}
	}
	return port
}

// TranslateServiceAddress is used to provide the final, translated address for a node,
// depending on how the agent and the other node are configured. The dc
// parameter is the dc the datacenter this node is from.
func (a *Agent) TranslateServiceAddress(dc string, addr string, taggedAddresses map[string]structs.ServiceAddress) string {
	if a.config.TranslateWANAddrs && (a.config.Datacenter != dc) {
		if wanAddr, ok := taggedAddresses["wan"]; ok && wanAddr.Address != "" {
			return wanAddr.Address
		}
	}
	return addr
}

// TranslateAddress is used to provide the final, translated address for a node,
// depending on how the agent and the other node are configured. The dc
// parameter is the dc the datacenter this node is from.
func (a *Agent) TranslateAddress(dc string, addr string, taggedAddresses map[string]string) string {
	if a.config.TranslateWANAddrs && (a.config.Datacenter != dc) {
		wanAddr := taggedAddresses["wan"]
		if wanAddr != "" {
			addr = wanAddr
		}
	}
	return addr
}

// TranslateAddresses translates addresses in the given structure into the
// final, translated address, depending on how the agent and the other node are
// configured. The dc parameter is the datacenter this structure is from.
func (a *Agent) TranslateAddresses(dc string, subj interface{}) {
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
			entry.Node.Address = a.TranslateAddress(dc, entry.Node.Address, entry.Node.TaggedAddresses)
			entry.Service.Address = a.TranslateServiceAddress(dc, entry.Service.Address, entry.Service.TaggedAddresses)
			entry.Service.Port = a.TranslateServicePort(dc, entry.Service.Port, entry.Service.TaggedAddresses)
		}
	case *structs.Node:
		v.Address = a.TranslateAddress(dc, v.Address, v.TaggedAddresses)
	case structs.Nodes:
		for _, node := range v {
			node.Address = a.TranslateAddress(dc, node.Address, node.TaggedAddresses)
		}
	case structs.ServiceNodes:
		for _, entry := range v {
			entry.Address = a.TranslateAddress(dc, entry.Address, entry.TaggedAddresses)
			entry.ServiceAddress = a.TranslateServiceAddress(dc, entry.ServiceAddress, entry.ServiceTaggedAddresses)
			entry.ServicePort = a.TranslateServicePort(dc, entry.ServicePort, entry.ServiceTaggedAddresses)
		}
	case *structs.NodeServices:
		if v.Node != nil {
			v.Node.Address = a.TranslateAddress(dc, v.Node.Address, v.Node.TaggedAddresses)
		}
		for _, entry := range v.Services {
			entry.Address = a.TranslateServiceAddress(dc, entry.Address, entry.TaggedAddresses)
			entry.Port = a.TranslateServicePort(dc, entry.Port, entry.TaggedAddresses)
		}
	default:
		panic(fmt.Errorf("Unhandled type passed to address translator: %#v", subj))
	}
}
