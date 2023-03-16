package tfgen

import (
	"text/template"

	"github.com/hashicorp/consul-topology/topology"
)

const squidInternalPort = 3128

func (g *Generator) getSquidContainer(
	net *topology.Network,
	ipAddress string,
) Resource {
	squid := struct {
		Name              string
		DockerNetworkName string
		InternalPort      int
		IPAddress         string
	}{
		Name:              net.Name,
		DockerNetworkName: net.DockerName,
		InternalPort:      squidInternalPort,
		IPAddress:         ipAddress,
	}

	return Eval(tfSquidT, &squid)
}

var tfSquidT = template.Must(template.ParseFS(content, "templates/container-squid.tf.tmpl"))
