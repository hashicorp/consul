package gce

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

type Config struct {
	// ProjectName is the name of the project where the the instances
	// should be discovered. It will be discovered if left empty.
	ProjectName string

	// ZonePattern is a regular expression for filtering zones.
	// Example: us-west1-.*, or us-(?west|east).*.
	ZonePattern string

	// TagValue contains the tag that matching instances must have.
	TagValue string

	// CredentialsFile is a path to a file with service account
	// credentials necessary to connect to GCE. If empty, the following
	// chain is respected:
	//
	//  1. A JSON file whose path is specified by the
	//     GOOGLE_APPLICATION_CREDENTIALS environment variable.
	//  2. A JSON file in a location known to the gcloud command-line tool.
	//     On Windows, this is %APPDATA%/gcloud/application_default_credentials.json.
	//     On other systems, $HOME/.config/gcloud/application_default_credentials.json.
	//  3. On Google Compute Engine, it fetches credentials from the metadata
	//     server.  (In this final case any provided scopes are ignored.)
	CredentialsFile string
}

// Discover returns the private ip addresses of all GCE instances in
// some or all zones of a project which have a certain tag.
func Discover(c *Config, l *log.Logger) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("[ERR] discover-gce: Missing configuration")
	}

	// determine the project name
	project := c.ProjectName
	if project == "" {
		l.Println("[INFO] discover-gce: Looking up project name")
		p, err := lookupProject()
		if err != nil {
			return nil, err
		}
		project = p
	}
	l.Printf("[INFO] discover-gce: Project name is %q", project)

	// create an authenticated client
	if c.CredentialsFile != "" {
		l.Printf("[INFO] discover-gce: Loading credentials from %s", c.CredentialsFile)
	}
	client, err := client(c.CredentialsFile)
	if err != nil {
		return nil, err
	}
	svc, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	// lookup the project zones to look in
	if c.ZonePattern != "" {
		l.Printf("[INFO] discover-gce: Looking up zones matching %s", c.ZonePattern)
	} else {
		l.Printf("[INFO] discover-gce: Looking up all zones")
	}
	zones, err := lookupZones(svc, project, c.ZonePattern)
	if err != nil {
		return nil, err
	}
	l.Printf("[INFO] discover-gce: Found zones %v", zones)

	// lookup the instance addresses
	var addrs []string
	for _, zone := range zones {
		a, err := lookupAddrs(svc, project, zone, c.TagValue)
		if err != nil {
			return nil, err
		}
		l.Printf("[INFO] discover-gce: Zone %q has %v", zone, a)
		addrs = append(addrs, a...)
	}
	return addrs, nil
}

// client returns an authenticated HTTP client for use with GCE.
func client(path string) (*http.Client, error) {
	if path == "" {
		return google.DefaultClient(oauth2.NoContext, compute.ComputeScope)
	}

	key, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	jwtConfig, err := google.JWTConfigFromJSON(key, compute.ComputeScope)
	if err != nil {
		return nil, err
	}

	return jwtConfig.Client(oauth2.NoContext), nil
}

// lookupProject retrieves the project name from the metadata of the current node.
func lookupProject() (string, error) {
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("discover-gce: invalid status code %d when fetching project id", resp.StatusCode)
	}

	project, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(project), nil
}

// lookupZones retrieves the zones of the project and filters them by pattern.
func lookupZones(svc *compute.Service, project, pattern string) ([]string, error) {
	call := svc.Zones.List(project)
	if pattern != "" {
		call = call.Filter("name eq " + pattern)
	}

	var zones []string
	f := func(page *compute.ZoneList) error {
		for _, v := range page.Items {
			zones = append(zones, v.Name)
		}
		return nil
	}

	if err := call.Pages(oauth2.NoContext, f); err != nil {
		return nil, err
	}
	return zones, nil
}

// lookupAddrs retrieves the private ip addresses of all instances in a given
// project and zone which have a matching tag value.
func lookupAddrs(svc *compute.Service, project, zone, tag string) ([]string, error) {
	var addrs []string
	f := func(page *compute.InstanceList) error {
		for _, v := range page.Items {
			if len(v.NetworkInterfaces) == 0 || v.NetworkInterfaces[0].NetworkIP == "" {
				continue
			}
			for _, t := range v.Tags.Items {
				if t == tag {
					addrs = append(addrs, v.NetworkInterfaces[0].NetworkIP)
					break
				}
			}
		}
		return nil
	}

	call := svc.Instances.List(project, zone)
	if err := call.Pages(oauth2.NoContext, f); err != nil {
		return nil, err
	}
	return addrs, nil
}
