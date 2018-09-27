package agent

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

func (a *Agent) sidecarServiceID(serviceID string) string {
	return serviceID + "-sidecar-proxy"
}

// sidecarServiceFromNodeService returns a *structs.NodeService representing a
// sidecar service with all defaults populated based on the current agent
// config.
//
// It assumes the ns has been validated already which means the nested
// SidecarService is also already validated.It also assumes that any check
// definitions within the sidecar service definition have been validated if
// necessary. If no sidecar service is defined in ns, then nil is returned with
// nil error.
//
// The second return argument is a list of CheckTypes to register along with the
// service.
//
// The third return argument is the effective Token to use for the sidecar
// registration. This will be the same as the token parameter passed unless the
// SidecarService definition contains a distint one.
func (a *Agent) sidecarServiceFromNodeService(ns *structs.NodeService, token string) (*structs.NodeService, []*structs.CheckType, string, error) {
	if ns.Connect.SidecarService == nil {
		return nil, nil, "", nil
	}

	// Start with normal conversion from service definition
	sidecar := ns.Connect.SidecarService.NodeService()

	// Override the ID which must always be consistent for a given outer service
	// ID. We rely on this for lifecycle management of the nested definition.
	sidecar.ID = a.sidecarServiceID(ns.ID)

	// Set some meta we can use to disambiguate between service instances we added
	// later and are responsible for deregistering.
	if sidecar.Meta != nil {
		// Meta is non-nil validate it before we add the special key so we can
		// enforce that user cannot add a consul- prefix one.
		if err := structs.ValidateMetadata(sidecar.Meta, false); err != nil {
			return nil, nil, "", err
		}
	}

	// Flag this as a sidecar - this is not persisted in catalog but only needed
	// in local agent state to disambiguate lineage when deregistereing the parent
	// service later.
	sidecar.LocallyRegisteredAsSidecar = true

	// See if there is a more specific token for the sidecar registration
	if ns.Connect.SidecarService.Token != "" {
		token = ns.Connect.SidecarService.Token
	}

	// Setup some sane connect proxy defaults.
	if sidecar.Kind == "" {
		sidecar.Kind = structs.ServiceKindConnectProxy
	}
	if sidecar.Service == "" {
		sidecar.Service = ns.Service + "-sidecar-proxy"
	}
	if sidecar.Address == "" {
		// Inherit address from the service if it's provided
		sidecar.Address = ns.Address
	}
	// Proxy defaults
	if sidecar.Proxy.DestinationServiceName == "" {
		sidecar.Proxy.DestinationServiceName = ns.Service
	}
	if sidecar.Proxy.DestinationServiceID == "" {
		sidecar.Proxy.DestinationServiceID = ns.ID
	}
	if sidecar.Proxy.LocalServiceAddress == "" {
		sidecar.Proxy.LocalServiceAddress = "127.0.0.1"
	}
	if sidecar.Proxy.LocalServicePort < 1 {
		sidecar.Proxy.LocalServicePort = ns.Port
	}

	// Allocate port if needed (min and max inclusive).
	rangeLen := a.config.ConnectSidecarMaxPort - a.config.ConnectSidecarMinPort + 1
	if sidecar.Port < 1 && a.config.ConnectSidecarMinPort > 0 && rangeLen > 0 {
		// This should be a really short list so don't bother optimising lookup yet.
	OUTER:
		for _, offset := range rand.Perm(rangeLen) {
			p := a.config.ConnectSidecarMinPort + offset
			// See if this port was already allocated to another service
			for _, otherNS := range a.State.Services() {
				if otherNS.Port == p {
					// already taken, skip to next random pick in the range
					continue OUTER
				}
			}
			// We made it through all existing proxies without a match so claim this one
			sidecar.Port = p
			break
		}
	}
	// If no ports left (or auto ports disabled) fail
	if sidecar.Port < 1 {
		// If ports are set to zero explicitly, config builder switches them to
		// `-1`. In this case don't show the actual values since we don't know what
		// was actually in config (zero or negative) and it might be confusing, we
		// just know they explicitly disabled auto assignment.
		if a.config.ConnectSidecarMinPort < 1 || a.config.ConnectSidecarMaxPort < 1 {
			return nil, nil, "", fmt.Errorf("no port provided for sidecar_service " +
				"and auto-assignement disabled in config")
		}
		return nil, nil, "", fmt.Errorf("no port provided for sidecar_service and none "+
			"left in the configured range [%d, %d]", a.config.ConnectSidecarMinPort,
			a.config.ConnectSidecarMaxPort)
	}

	// Setup checks
	checks, err := ns.Connect.SidecarService.CheckTypes()
	if err != nil {
		return nil, nil, "", err
	}

	// Setup default check if none given
	if len(checks) < 1 {
		checks = []*structs.CheckType{
			&structs.CheckType{
				Name: "Connect Sidecar Listening",
				// Default to localhost rather than agent/service public IP. The checks
				// can always be overridden if a non-loopback IP is needed.
				TCP:      fmt.Sprintf("127.0.0.1:%d", sidecar.Port),
				Interval: 10 * time.Second,
			},
			&structs.CheckType{
				Name:         "Connect Sidecar Aliasing " + ns.ID,
				AliasService: ns.ID,
			},
		}
	}

	return sidecar, checks, token, nil
}
