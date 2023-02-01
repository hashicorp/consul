package export

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	serviceName    string
	peerNames      string
	partitionNames string
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if err := c.validateFlags(); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	var peerNames []string
	if c.peerNames != "" {
		peerNames = strings.Split(c.peerNames, ",")
		for _, peerName := range peerNames {
			if peerName == "" {
				c.UI.Error(fmt.Sprintf("Invalid peer %q", peerName))
				return 1
			}
		}
	}

	partitionNames, err := c.getPartitionNames()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	entry, _, err := client.ConfigEntries().Get("exported-services", "default", nil)
	if err != nil && !strings.Contains(err.Error(), agent.ConfigEntryNotFoundErr) {
		c.UI.Error(fmt.Sprintf("Error reading config entry %s/%s: %v", "exported-services", "default", err))
		return 1
	}
	if entry == nil {
		cfg := api.ExportedServicesConfigEntry{
			Name: "default",
			Services: []api.ExportedService{
				{
					Name:      c.serviceName,
					Consumers: buildConsumers(peerNames, partitionNames),
				},
			},
		}
		c.UI.Info(fmt.Sprintf("%#v", cfg))
		_, _, err = client.ConfigEntries().Set(&cfg, &api.WriteOptions{})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error creating config entry: %s", err))
			return 1
		}

	} else {
		cfg, ok := entry.(*api.ExportedServicesConfigEntry)
		if !ok {
			c.UI.Error(fmt.Sprintf("Existing config entry has incorrect type: %t", entry))
			return 1
		}

		serviceExists := false

		for i, service := range cfg.Services {
			if service.Name == c.serviceName {
				serviceExists = true

				// Add a consumer for each peer where one doesn't already exist
				for _, peerName := range peerNames {
					peerExists := false
					for _, consumer := range service.Consumers {
						if consumer.Peer == peerName {
							peerExists = true
							break
						}
					}
					if !peerExists {
						cfg.Services[i].Consumers = append(cfg.Services[i].Consumers, api.ServiceConsumer{Peer: peerName})
					}
				}

				// Add a consumer for each partition where one doesn't already exist
				for _, partitionName := range partitionNames {
					partitionExists := false

					for _, consumer := range service.Consumers {
						if consumer.Partition == partitionName {
							partitionExists = true
							break
						}
					}
					if !partitionExists {
						cfg.Services[i].Consumers = append(cfg.Services[i].Consumers, api.ServiceConsumer{Partition: partitionName})
					}
				}
			}
		}

		if !serviceExists {
			cfg.Services = append(cfg.Services, api.ExportedService{
				Name:      c.serviceName,
				Consumers: buildConsumers(peerNames, partitionNames),
			})
		}

		ok, _, err := client.ConfigEntries().CAS(cfg, cfg.GetModifyIndex(), nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error writing modifed service and peer to config entry: %s", err))
			return 1
		}

		if !ok {
			c.UI.Error(fmt.Sprintf("Modifed service and peer config entry was changed during update. Please try again"))
			return 1
		}
	}

	if len(c.peerNames) > 0 && len(c.partitionNames) > 0 {
		c.UI.Info(fmt.Sprintf("Successfully exported service %q to peers %q and to partitions %q", c.serviceName, c.peerNames, c.partitionNames))
	} else if len(c.peerNames) > 0 {
		c.UI.Info(fmt.Sprintf("Successfully exported service %q to peers %q", c.serviceName, c.peerNames))
	} else if len(c.partitionNames) > 0 {
		c.UI.Info(fmt.Sprintf("Successfully exported service %q to partitions %q", c.serviceName, c.partitionNames))
	}
	return 0
}

func buildConsumers(peerNames []string, partitionNames []string) []api.ServiceConsumer {
	var consumers []api.ServiceConsumer
	for _, peer := range peerNames {
		consumers = append(consumers, api.ServiceConsumer{
			Peer: peer,
		})
	}
	for _, partition := range partitionNames {
		consumers = append(consumers, api.ServiceConsumer{
			Partition: partition,
		})
	}
	return consumers
}

//========

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Export a service"
	help     = `
Usage: consul services export [options] -name <service name> -consumer-peers <other cluster name>

  Export a service. The peers provided will be used locally by
  this cluster to refer to the other cluster where the services will be exported. 

  Example:

  $ consul services export -name=web -consumer-peers=other-cluster
`
)
