package agent

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

// discoverGCEHosts searches a Google Compute Engine region, returning a list
// of instance ips that match the tags given in GCETags.
func (c *Config) discoverGCEHosts(logger *log.Logger) ([]string, error) {
	config := c.RetryJoinGCE
	ctx := oauth2.NoContext
	var client *http.Client
	var err error

	logger.Printf("[INFO] agent: Initializing GCE client")
	if config.CredentialsFile != "" {
		logger.Printf("[INFO] agent: Loading credentials from %s", config.CredentialsFile)
		key, err := ioutil.ReadFile(config.CredentialsFile)
		if err != nil {
			return nil, err
		}
		jwtConfig, err := google.JWTConfigFromJSON(key, compute.ComputeScope)
		if err != nil {
			return nil, err
		}
		client = jwtConfig.Client(ctx)
	} else {
		logger.Printf("[INFO] agent: Using default credential chain")
		client, err = google.DefaultClient(ctx, compute.ComputeScope)
		if err != nil {
			return nil, err
		}
	}

	computeService, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	if config.ProjectName == "" {
		logger.Printf("[INFO] agent: No GCE project provided, will discover from metadata.")
		config.ProjectName, err = gceProjectIDFromMetadata(logger)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Printf("[INFO] agent: Using pre-defined GCE project name: %s", config.ProjectName)
	}

	zones, err := gceDiscoverZones(ctx, logger, computeService, config.ProjectName, config.ZonePattern)
	if err != nil {
		return nil, err
	}

	logger.Printf("[INFO] agent: Discovering GCE hosts with tag %s in zones: %s", config.TagValue, strings.Join(zones, ", "))

	var servers []string
	for _, zone := range zones {
		addresses, err := gceInstancesAddressesForZone(ctx, logger, computeService, config.ProjectName, zone, config.TagValue)
		if err != nil {
			return nil, err
		}
		if len(addresses) > 0 {
			logger.Printf("[INFO] agent: Discovered %d instances in %s/%s: %v", len(addresses), config.ProjectName, zone, addresses)
		}
		servers = append(servers, addresses...)
	}

	return servers, nil
}

// gceProjectIDFromMetadata queries the metadata service on GCE to get the
// project ID (name) of an instance.
func gceProjectIDFromMetadata(logger *log.Logger) (string, error) {
	logger.Printf("[INFO] agent: Attempting to discover GCE project from metadata.")
	client := &http.Client{}

	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	project, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	logger.Printf("[INFO] agent: GCE project discovered as: %s", project)
	return string(project), nil
}

// gceDiscoverZones discovers a list of zones from a supplied zone pattern, or
// all of the zones available to a project.
func gceDiscoverZones(ctx context.Context, logger *log.Logger, computeService *compute.Service, project, pattern string) ([]string, error) {
	var zones []string

	if pattern != "" {
		logger.Printf("[INFO] agent: Discovering zones for project %s matching pattern: %s", project, pattern)
	} else {
		logger.Printf("[INFO] agent: Discovering all zones available to project: %s", project)
	}

	call := computeService.Zones.List(project)
	if pattern != "" {
		call = call.Filter(fmt.Sprintf("name eq %s", pattern))
	}

	if err := call.Pages(ctx, func(page *compute.ZoneList) error {
		for _, v := range page.Items {
			zones = append(zones, v.Name)
		}
		return nil
	}); err != nil {
		return zones, err
	}

	logger.Printf("[INFO] agent: Discovered GCE zones: %s", strings.Join(zones, ", "))
	return zones, nil
}

// gceInstancesAddressesForZone locates all instances within a specific project
// and zone, matching the supplied tag. Only the private IP addresses are
// returned, but ID is also logged.
func gceInstancesAddressesForZone(ctx context.Context, logger *log.Logger, computeService *compute.Service, project, zone, tag string) ([]string, error) {
	var addresses []string
	call := computeService.Instances.List(project, zone)
	if err := call.Pages(ctx, func(page *compute.InstanceList) error {
		for _, v := range page.Items {
			for _, t := range v.Tags.Items {
				if t == tag && len(v.NetworkInterfaces) > 0 && v.NetworkInterfaces[0].NetworkIP != "" {
					addresses = append(addresses, v.NetworkInterfaces[0].NetworkIP)
				}
			}
		}
		return nil
	}); err != nil {
		return addresses, err
	}

	return addresses, nil
}
