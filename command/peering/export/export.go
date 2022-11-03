package export

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

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

	serviceName string
	peerName    string
}

func (c *cmd) init() {
	//This function defines the flags

	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.serviceName, "serviceName", "", "(Required) Specify the name of the service you want to export.")
	c.flags.StringVar(&c.peerName, "peerName", "", "(Required) Specify the name of the peer you want to export.")

	// c.flags.Var((*flags.FlagMapValue)(&c.meta), "meta",
	// 	"Metadata to associate with the peering, formatted as key=value. This flag "+
	// 		"may be specified multiple times to set multiple metadata fields.")

	// c.flags.Var((*flags.AppendSliceValue)(&c.peer), "peer",
	// 	"A list of peers where the services will be exported")

	// c.flags.StringVar(
	// 	&c.format,
	// 	"format",
	// 	peering.PeeringFormatPretty,
	// 	fmt.Sprintf("Output format {%s} (default: %s)", strings.Join(peering.GetSupportedFormats(), "|"), peering.PeeringFormatPretty),
	// )

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.serviceName == "" {
		c.UI.Error("Missing the required -service name flag")
		return 1
	}

	if c.peerName == "" {
		c.UI.Error("Missing the required -peer name flag")
		return 1
	}

	//if !peering.FormatIsValid(c.format) {
	//c.UI.Error(fmt.Sprintf("Invalid format, valid formats are {%s}", strings.Join(peering.GetSupportedFormats(), "|")))
	//	return 1
	//}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	entry, _, err := client.ConfigEntries().Get("exported-services", "default", nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading config entry %s/%s: %v", "exported-services", "default", err))
		return 1
	}
	if entry == nil {
		cfg := api.ExportedServicesConfigEntry{
			Name: "default",
			Services: []api.ExportedService{
				{
					Name: c.serviceName,
					Consumers: []api.ServiceConsumer{
						{
							Peer: c.peerName,
						},
					},
				},
			},
		}
		_, _, err = client.ConfigEntries().Set(&cfg, &api.WriteOptions{})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error creating config entry: %s", err))
			return 1
		}

	} else {
		c.UI.Info(fmt.Sprintf("We found an existing config entry %s/%s: %+v", "exported-services", "default", entry))

		cfg, ok := entry.(*api.ExportedServicesConfigEntry) 
		if !ok {
			c.UI.Error(fmt.Sprintf("Existing config entry has incorrect type: %t", entry))
			return 1
		}

		for i, service := range cfg.Services {

			if (service.Name == c.serviceName){
			
				for _, consumer := range service.Consumers{
					if (consumer.Peer == c.peerName){
						c.UI.Info(fmt.Sprintf("We found an existing service entry with the provided peer"))
						return 0
					}
				
				}
				c.UI.Info(fmt.Sprintf("We found an existing service entry %+v", cfg))

				cfg.Services[i].Consumers = append(cfg.Services[i].Consumers, api.ServiceConsumer{Peer: c.peerName})

				ok, _, err := client.ConfigEntries().CAS(cfg, cfg.GetModifyIndex(), nil)
				if err != nil{
						c.UI.Error(fmt.Sprintf("error writing modifed value to config entry: %s", err))
						return 1
				}

				if !ok {
					c.UI.Error(fmt.Sprintf("config entry was changed during update. Please try again"))
						return 1
				}
				c.UI.Info(fmt.Sprintf("We modified the service entry %+v", cfg))
				return 0
			}

		}
		cfg.Services = append(cfg.Services, api.ExportedService{
				Name: c.serviceName,
				Consumers: []api.ServiceConsumer{
					{
						Peer: c.peerName,
					},
				},
			},
		)

		ok, _, err := client.ConfigEntries().CAS(cfg, cfg.GetModifyIndex(), nil)
				if err != nil{
						c.UI.Error(fmt.Sprintf("error writing modifed service and peer to config entry: %s", err))
						return 1
				}

				if !ok {
					c.UI.Error(fmt.Sprintf("modifed service and peer config entry was changed during update. Please try again"))
						return 1
				}
				c.UI.Info(fmt.Sprintf("We modified the modifed service and peer entry %+v", cfg))
				return 0
		
		
	}
	
	
	return 0
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
Usage: consul peering export [options] -service <service name> -peers <other cluster name>

  Export a service. The peers provided will be used locally by
  this cluster to refer to the other cluster where the services will be exported. 

  Example:

	$ consul peering export -service=web -peers=other-cluster
`
)
