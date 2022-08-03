package agent

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/ipaddr"

	"github.com/hashicorp/consul/agent/structs"
)

func sidecarServiceID(serviceID string) string {
	return serviceID + "-sidecar-proxy"
}

// sidecarServiceFromNodeService returns a *structs.NodeService representing a
// sidecar service with all defaults populated based on the current agent
// config.
//
// It assumes the ns has been validated already which means the nested
// SidecarService is also already validated. It also assumes that any check
// definitions within the sidecar service definition have been validated if
// necessary. If no sidecar service is defined in ns, then nil is returned with
// nil error.
//
// The second return argument is a list of CheckTypes to register along with the
// service.
//
// The third return argument is the effective Token to use for the sidecar
// registration. This will be the same as the token parameter passed unless the
// SidecarService definition contains a distinct one.
// TODO: return AddServiceRequest
func (a *Agent) sidecarServiceFromNodeService(ns *structs.NodeService, token string) (*structs.NodeService, []*structs.CheckType, string, error) {
	if ns.Connect.SidecarService == nil {
		return nil, nil, "", nil
	}

	// for now at least these must be identical
	ns.Connect.SidecarService.EnterpriseMeta = ns.EnterpriseMeta

	// Start with normal conversion from service definition
	sidecar := ns.Connect.SidecarService.NodeService()

	// Override the ID which must always be consistent for a given outer service
	// ID. We rely on this for lifecycle management of the nested definition.
	sidecar.ID = sidecarServiceID(ns.ID)

	// Set some meta we can use to disambiguate between service instances we added
	// later and are responsible for deregistering.
	if sidecar.Meta != nil {
		// Meta is non-nil validate it before we add the special key so we can
		// enforce that user cannot add a consul- prefix one.
		if err := structs.ValidateServiceMetadata(sidecar.Kind, sidecar.Meta, false); err != nil {
			return nil, nil, "", err
		}
	}

	// Copy the service metadata from the original service if no other meta was provided
	if len(sidecar.Meta) == 0 && len(ns.Meta) > 0 {
		if sidecar.Meta == nil {
			sidecar.Meta = make(map[string]string)
		}
		for k, v := range ns.Meta {
			sidecar.Meta[k] = v
		}
	}

	// Copy the tags from the original service if no other tags were specified
	if len(sidecar.Tags) == 0 && len(ns.Tags) > 0 {
		sidecar.Tags = append(sidecar.Tags, ns.Tags...)
	}

	// Flag this as a sidecar - this is not persisted in catalog but only needed
	// in local agent state to disambiguate lineage when deregistering the parent
	// service later.
	sidecar.LocallyRegisteredAsSidecar = true

	// See if there is a more specific token for the sidecar registration
	if ns.Connect.SidecarService.Token != "" {
		token = ns.Connect.SidecarService.Token
	}

	// Setup some reasonable connect proxy defaults.
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

	// Fill defaults from NodeService if none of the address components are present.
	// This really argues for a refactoring to a more generalized 'address' concept.
	if sidecar.Proxy.LocalServiceSocketPath == "" && (sidecar.Proxy.LocalServiceAddress == "" || sidecar.Proxy.LocalServicePort < 1) {
		if ns.SocketPath != "" {
			sidecar.Proxy.LocalServiceSocketPath = ns.SocketPath
		} else {
			if sidecar.Proxy.LocalServiceAddress == "" {
				sidecar.Proxy.LocalServiceAddress = "127.0.0.1"
			}
			if sidecar.Proxy.LocalServicePort < 1 {
				sidecar.Proxy.LocalServicePort = ns.Port
			}
		}
	}

	if sidecar.Port < 1 {
		port, err := a.sidecarPortFromServiceID(sidecar.CompoundServiceID())
		if err != nil {
			return nil, nil, "", err
		}
		sidecar.Port = port
	}

	// Setup checks
	checks, err := ns.Connect.SidecarService.CheckTypes()
	if err != nil {
		return nil, nil, "", err
	}
	// Setup default check if none given
	if len(checks) < 1 {
		checks = sidecarDefaultChecks(ns.ID, sidecar.Proxy.LocalServiceAddress, sidecar.Port)
	}

	return sidecar, checks, token, nil
}

// sidecarPortFromServiceID is used to allocate a unique port for a sidecar proxy.
// This is called immediately before registration to avoid value collisions. This function assumes the state lock is already held.
func (a *Agent) sidecarPortFromServiceID(sidecarCompoundServiceID structs.ServiceID) (int, error) {
	sidecarPort := 0

	// Allocate port if needed (min and max inclusive).
	rangeLen := a.config.ConnectSidecarMaxPort - a.config.ConnectSidecarMinPort + 1
	if sidecarPort < 1 && a.config.ConnectSidecarMinPort > 0 && rangeLen > 0 {
		// This did pick at random which was simpler but consul reload would assign
		// new ports to all the sidecars since it unloads all state and
		// re-populates. It also made this more difficult to test (have to pin the
		// range to one etc.). Instead we assign sequentially, but rather than N^2
		// lookups, just iterated services once and find the set of used ports in
		// allocation range. We could maintain this state permanently in agent but
		// it doesn't seem to be necessary - even with thousands of services this is
		// not expensive to compute.
		usedPorts := make(map[int]struct{})
		for _, otherNS := range a.State.AllServices() {
			// Check if other port is in auto-assign range
			if otherNS.Port >= a.config.ConnectSidecarMinPort &&
				otherNS.Port <= a.config.ConnectSidecarMaxPort {
				if otherNS.CompoundServiceID() == sidecarCompoundServiceID {
					// This sidecar is already registered with an auto-port and is just
					// being updated so pick the same port as before rather than allocate
					// a new one.
					sidecarPort = otherNS.Port
					break
				}
				usedPorts[otherNS.Port] = struct{}{}
			}
			// Note that the proxy might already be registered with a port that was
			// not in the auto range or the auto range has moved. In either case we
			// want to allocate a new one so it's no different from ignoring that it
			// already exists as we do now.
		}

		// Check we still need to assign a port and didn't find we already had one
		// allocated.
		if sidecarPort < 1 {
			// Iterate until we find lowest unused port
			for p := a.config.ConnectSidecarMinPort; p <= a.config.ConnectSidecarMaxPort; p++ {
				_, used := usedPorts[p]
				if !used {
					sidecarPort = p
					break
				}
			}
		}
	}
	// If no ports left (or auto ports disabled) fail
	if sidecarPort < 1 {
		// If ports are set to zero explicitly, config builder switches them to
		// `-1`. In this case don't show the actual values since we don't know what
		// was actually in config (zero or negative) and it might be confusing, we
		// just know they explicitly disabled auto assignment.
		if a.config.ConnectSidecarMinPort < 1 || a.config.ConnectSidecarMaxPort < 1 {
			return 0, fmt.Errorf("no port provided for sidecar_service " +
				"and auto-assignment disabled in config")
		}
		return 0, fmt.Errorf("no port provided for sidecar_service and none "+
			"left in the configured range [%d, %d]", a.config.ConnectSidecarMinPort,
			a.config.ConnectSidecarMaxPort)
	}

	return sidecarPort, nil
}

func sidecarDefaultChecks(serviceID string, localServiceAddress string, port int) []*structs.CheckType {
	// Setup default check if none given
	return []*structs.CheckType{
		{
			Name: "Connect Sidecar Listening",
			// Default to localhost rather than agent/service public IP. The checks
			// can always be overridden if a non-loopback IP is needed.
			TCP:      ipaddr.FormatAddressPort(localServiceAddress, port),
			Interval: 10 * time.Second,
		},
		{
			Name:         "Connect Sidecar Aliasing " + serviceID,
			AliasService: serviceID,
		},
	}
}
