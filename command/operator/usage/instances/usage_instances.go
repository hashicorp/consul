// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package instances

import (
	"bytes"
	"flag"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
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

	// flags
	onlyBillable   bool
	onlyConnect    bool
	allDatacenters bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.onlyBillable, "billable", false, "Display only billable service info. "+
		"Cannot be used with -connect.")
	c.flags.BoolVar(&c.onlyConnect, "connect", false, "Display only Connect service info."+
		"Cannot be used with -billable.")
	c.flags.BoolVar(&c.allDatacenters, "all-datacenters", false, "Display service counts from "+
		"all datacenters.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if l := len(c.flags.Args()); l > 0 {
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 0, got %d)", l))
		return 1
	}

	if c.onlyBillable && c.onlyConnect {
		c.UI.Error("Cannot specify both -billable and -connect flags")
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	billableTotal := 0
	var datacenterBillableTotals []string
	usage, _, err := client.Operator().Usage(&api.QueryOptions{Global: c.allDatacenters})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error fetching usage information: %s", err))
		return 1
	}
	for dc, usage := range usage.Usage {
		billableTotal += usage.BillableServiceInstances
		datacenterBillableTotals = append(datacenterBillableTotals,
			fmt.Sprintf("%s Billable Service Instances: %d", dc, usage.BillableServiceInstances))
	}

	// Output billable service counts
	if !c.onlyConnect {
		c.UI.Output(fmt.Sprintf("Billable Service Instances Total: %d", billableTotal))
		sort.Strings(datacenterBillableTotals)
		for _, datacenterTotal := range datacenterBillableTotals {
			c.UI.Output(datacenterTotal)
		}

		c.UI.Output("\nBillable Services")
		billableOutput, err := formatServiceCounts(usage.Usage, true, c.allDatacenters)
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		c.UI.Output(billableOutput + "\n")
	}

	// Output Connect service counts
	if !c.onlyBillable {
		c.UI.Output("Connect Services")
		connectOutput, err := formatServiceCounts(usage.Usage, false, c.allDatacenters)
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		c.UI.Output(connectOutput)
	}

	return 0
}

func formatServiceCounts(usageStats map[string]api.ServiceUsage, billable, showDatacenter bool) (string, error) {
	var output bytes.Buffer
	tw := tabwriter.NewWriter(&output, 0, 2, 6, ' ', 0)
	var serviceCounts []serviceCount

	for datacenter, usage := range usageStats {
		if billable {
			serviceCounts = append(serviceCounts, getBillableInstanceCounts(usage, datacenter)...)
		} else {
			serviceCounts = append(serviceCounts, getConnectInstanceCounts(usage, datacenter)...)
		}
	}

	sortServiceCounts(serviceCounts)

	if showDatacenter {
		fmt.Fprintf(tw, "Datacenter\t")
	}
	if showPartitionNamespace {
		fmt.Fprintf(tw, "Partition\tNamespace\t")
	}
	if !billable {
		fmt.Fprintf(tw, "Type\t")
	} else {
		fmt.Fprintf(tw, "Services\t")
	}
	fmt.Fprintf(tw, "Service instances\n")

	serviceTotal := 0
	instanceTotal := 0
	for _, c := range serviceCounts {
		if showDatacenter {
			fmt.Fprintf(tw, "%s\t", c.datacenter)
		}
		if showPartitionNamespace {
			fmt.Fprintf(tw, "%s\t%s\t", c.partition, c.namespace)
		}
		if !billable {
			fmt.Fprintf(tw, "%s\t", c.serviceType)
		} else {
			fmt.Fprintf(tw, "%d\t", c.services)
		}
		fmt.Fprintf(tw, "%d\n", c.instanceCount)

		serviceTotal += c.services
		instanceTotal += c.instanceCount
	}

	// Show total counts if there's multiple rows because of datacenter or partition/ns view
	if showDatacenter || showPartitionNamespace {
		if showDatacenter {
			fmt.Fprint(tw, "\t")
		}
		if showPartitionNamespace {
			fmt.Fprint(tw, "\t\t")
		}
		fmt.Fprint(tw, "\t\n")
		fmt.Fprintf(tw, "Total")
		if showPartitionNamespace {
			fmt.Fprint(tw, "\t")
			if showDatacenter {
				fmt.Fprint(tw, "\t")
			}
		}

		if billable {
			fmt.Fprintf(tw, "\t%d\t%d\n", serviceTotal, instanceTotal)
		} else {
			fmt.Fprintf(tw, "\t\t%d\n", instanceTotal)
		}
	}

	if err := tw.Flush(); err != nil {
		return "", fmt.Errorf("Error flushing tabwriter: %s", err)
	}
	return strings.TrimSpace(output.String()), nil
}

type serviceCount struct {
	datacenter    string
	partition     string
	namespace     string
	serviceType   string
	instanceCount int
	services      int
}

// Sort entries by datacenter > partition > namespace
func sortServiceCounts(counts []serviceCount) {
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].datacenter != counts[j].datacenter {
			return counts[i].datacenter < counts[j].datacenter
		}
		if counts[i].partition != counts[j].partition {
			return counts[i].partition < counts[j].partition
		}
		if counts[i].namespace != counts[j].namespace {
			return counts[i].namespace < counts[j].namespace
		}
		return counts[i].serviceType < counts[j].serviceType
	})
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Display service instance usage information"
	help     = `
Usage: consul operator usage instances [options]

  Retrieves usage information about the number of services registered in a given
  datacenter. By default, the datacenter of the local agent is queried.

  To retrieve the service usage data:

      $ consul operator usage instances

  To show only billable service instance counts:

      $ consul operator usage instances -billable

  To show only connect service instance counts:

      $ consul operator usage instances -connect

  For a full list of options and examples, please see the Consul documentation.
`
)
