package export

import (
	"errors"
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

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.serviceName, "name", "", "(Required) Specify the name of the service you want to export.")
	c.flags.StringVar(&c.peerNames, "consumer-peers", "", "(Required) A comma-separated list of cluster peers to export the service to. In Consul Enterprise, this flag is optional if -consumer-partitions is specified.")
	c.flags.StringVar(&c.partitionNames, "consumer-partitions", "", "(Enterprise only) A comma-separated list of admin partitions within the same datacenter to export the service to. This flag is optional if -consumer-peers is specified.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if err := c.validateFlags(); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	peerNames, err := c.getPeerNames()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
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

	// Name matches partition, so "default" if none specified
	cfgName := "default"
	if c.http.Partition() != "" {
		cfgName = c.http.Partition()
	}

	entry, _, err := client.ConfigEntries().Get(api.ExportedServices, cfgName, &api.QueryOptions{Namespace: ""})
	if err != nil && !strings.Contains(err.Error(), agent.ConfigEntryNotFoundErr) {
		c.UI.Error(fmt.Sprintf("Error reading config entry %s/%s: %v", "exported-services", "default", err))
		return 1
	}

	var cfg *api.ExportedServicesConfigEntry
	if entry == nil {
		cfg = c.initializeConfigEntry(cfgName, peerNames, partitionNames)
	} else {
		existingCfg, ok := entry.(*api.ExportedServicesConfigEntry)
		if !ok {
			c.UI.Error(fmt.Sprintf("Existing config entry has incorrect type: %t", entry))
			return 1
		}

		cfg = c.updateConfigEntry(existingCfg, peerNames, partitionNames)
	}

	ok, _, err := client.ConfigEntries().CAS(cfg, cfg.GetModifyIndex(), nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing config entry: %s", err))
		return 1
	} else if !ok {
		c.UI.Error(fmt.Sprintf("Config entry was changed during update. Please try again"))
		return 1
	}

	switch {
	case len(c.peerNames) > 0 && len(c.partitionNames) > 0:
		c.UI.Info(fmt.Sprintf("Successfully exported service %q to cluster peers %q and to partitions %q", c.serviceName, c.peerNames, c.partitionNames))
	case len(c.peerNames) > 0:
		c.UI.Info(fmt.Sprintf("Successfully exported service %q to cluster peers %q", c.serviceName, c.peerNames))
	case len(c.partitionNames) > 0:
		c.UI.Info(fmt.Sprintf("Successfully exported service %q to partitions %q", c.serviceName, c.partitionNames))
	}

	return 0
}

func (c *cmd) validateFlags() error {
	if c.serviceName == "" {
		return errors.New("Missing the required -name flag")
	}

	if c.peerNames == "" && c.partitionNames == "" {
		return errors.New("Missing the required -consumer-peers or -consumer-partitions flag")
	}

	return nil
}

func (c *cmd) getPeerNames() ([]string, error) {
	var peerNames []string
	if c.peerNames != "" {
		peerNames = strings.Split(c.peerNames, ",")
		for _, peerName := range peerNames {
			if peerName == "" {
				return nil, fmt.Errorf("Invalid peer %q", peerName)
			}
		}
	}
	return peerNames, nil
}

func (c *cmd) getPartitionNames() ([]string, error) {
	var partitionNames []string
	if c.partitionNames != "" {
		partitionNames = strings.Split(c.partitionNames, ",")
		for _, partitionName := range partitionNames {
			if partitionName == "" {
				return nil, fmt.Errorf("Invalid partition %q", partitionName)
			}
		}
	}
	return partitionNames, nil
}

func (c *cmd) initializeConfigEntry(cfgName string, peerNames, partitionNames []string) *api.ExportedServicesConfigEntry {
	return &api.ExportedServicesConfigEntry{
		Name: cfgName,
		Services: []api.ExportedService{
			{
				Name:      c.serviceName,
				Namespace: c.http.Namespace(),
				Consumers: buildConsumers(peerNames, partitionNames),
			},
		},
	}
}

func (c *cmd) updateConfigEntry(cfg *api.ExportedServicesConfigEntry, peerNames, partitionNames []string) *api.ExportedServicesConfigEntry {
	serviceExists := false

	for i, service := range cfg.Services {
		if service.Name == c.serviceName && service.Namespace == c.http.Namespace() {
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
			Namespace: c.http.Namespace(),
			Consumers: buildConsumers(peerNames, partitionNames),
		})
	}

	return cfg
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
	synopsis = "Export a service from one peer or admin partition to another"
	help     = `
Usage: consul services export [options] -name <service name> -consumer-peers <other cluster name>

  Export a service to a peered cluster.

      $ consul services export -name=web -consumer-peers=other-cluster

  Use the -consumer-partitions flag instead of -consumer-peers to export to a different partition in the same cluster.

      $ consul services export -name=web -consumer-partitions=other-partition

  Additional flags and more advanced use cases are detailed below.
`
)
