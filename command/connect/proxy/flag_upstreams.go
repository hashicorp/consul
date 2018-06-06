package proxy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/connect/proxy"
)

// FlagUpstreams implements the flag.Value interface and allows specifying
// the -upstream flag multiple times and keeping track of the name of the
// upstream and the local port.
//
// The syntax of the value is "name:addr" where addr can be "port" or
// "host:port". Examples: "db:8181", "db:127.0.0.10:8282", etc.
type FlagUpstreams map[string]proxy.UpstreamConfig

func (f *FlagUpstreams) String() string {
	return fmt.Sprintf("%v", *f)
}

func (f *FlagUpstreams) Set(value string) error {
	idx := strings.Index(value, ":")
	if idx == -1 {
		return fmt.Errorf("Upstream value should be name:addr in %q", value)
	}

	addr := ""
	name := value[:idx]
	portRaw := value[idx+1:]
	if idx := strings.Index(portRaw, ":"); idx != -1 {
		addr = portRaw[:idx]
		portRaw = portRaw[idx+1:]
	}

	destinationType := "service"
	if idx := strings.Index(name, "."); idx != -1 {
		typ := name[idx+1:]
		name = name[:idx]
		switch typ {
		case "", "service":
			destinationType = "service"

		case "query":
			destinationType = "prepared_query"

		default:
			return fmt.Errorf(
				"Upstream type must be blank, 'service', or 'query'. Got: %q", typ)
		}
	}

	port, err := strconv.ParseInt(portRaw, 0, 0)
	if err != nil {
		return err
	}

	if *f == nil {
		*f = make(map[string]proxy.UpstreamConfig)
	}

	(*f)[name] = proxy.UpstreamConfig{
		LocalBindAddress: addr,
		LocalBindPort:    int(port),
		DestinationName:  name,
		DestinationType:  destinationType,
	}

	return nil
}
