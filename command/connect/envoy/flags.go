package envoy

import (
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-sockaddr/template"
)

const defaultMeshGatewayPort int = 443

// ServiceAddressValue implements a flag.Value that may be used to parse an
// addr:port string into an api.ServiceAddress.
type ServiceAddressValue struct {
	value api.ServiceAddress
}

func (s *ServiceAddressValue) String() string {
	if s == nil {
		return fmt.Sprintf(":%d", defaultMeshGatewayPort)
	}
	return fmt.Sprintf("%v:%d", s.value.Address, s.value.Port)
}

func (s *ServiceAddressValue) Value() api.ServiceAddress {
	if s == nil || s.value.Port == 0 && s.value.Address == "" {
		return api.ServiceAddress{Port: defaultMeshGatewayPort}
	}
	return s.value
}

func (s *ServiceAddressValue) Set(raw string) error {
	x, err := template.Parse(raw)
	if err != nil {
		return fmt.Errorf("Error parsing address %q: %v", raw, err)
	}

	addr, portStr, err := net.SplitHostPort(x)
	if err != nil {
		return fmt.Errorf("Error parsing address %q: %v", x, err)
	}

	port := defaultMeshGatewayPort
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("Error parsing port %q: %v", portStr, err)
		}
	}

	s.value.Address = addr
	s.value.Port = port
	return nil
}

var _ flag.Value = &ServiceAddressValue{}
