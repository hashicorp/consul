package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/sync/errgroup"
)

// ScalewayServer represents a Scaleway server
type ScalewayServer struct {
	// Arch is the architecture target of the server
	Arch string `json:"arch,omitempty"`

	// Identifier is a unique identifier for the server
	Identifier string `json:"id,omitempty"`

	// Name is the user-defined name of the server
	Name string `json:"name,omitempty"`

	// CreationDate is the creation date of the server
	CreationDate string `json:"creation_date,omitempty"`

	// ModificationDate is the date of the last modification of the server
	ModificationDate string `json:"modification_date,omitempty"`

	// Image is the image used by the server
	Image ScalewayImage `json:"image,omitempty"`

	// DynamicIPRequired is a flag that defines a server with a dynamic ip address attached
	DynamicIPRequired *bool `json:"dynamic_ip_required,omitempty"`

	// PublicIP is the public IP address bound to the server
	PublicAddress ScalewayIPAddress `json:"public_ip,omitempty"`

	// State is the current status of the server
	State string `json:"state,omitempty"`

	// StateDetail is the detailed status of the server
	StateDetail string `json:"state_detail,omitempty"`

	// PrivateIP represents the private IPV4 attached to the server (changes on each boot)
	PrivateIP string `json:"private_ip,omitempty"`

	// Bootscript is the unique identifier of the selected bootscript
	Bootscript *ScalewayBootscript `json:"bootscript,omitempty"`

	// Hostname represents the ServerName in a format compatible with unix's hostname
	Hostname string `json:"hostname,omitempty"`

	// Tags represents user-defined tags
	Tags []string `json:"tags,omitempty"`

	// Volumes are the attached volumes
	Volumes map[string]ScalewayVolume `json:"volumes,omitempty"`

	// SecurityGroup is the selected security group object
	SecurityGroup ScalewaySecurityGroup `json:"security_group,omitempty"`

	// Organization is the owner of the server
	Organization string `json:"organization,omitempty"`

	// CommercialType is the commercial type of the server (i.e: C1, C2[SML], VC1S)
	CommercialType string `json:"commercial_type,omitempty"`

	// Location of the server
	Location struct {
		Platform   string `json:"platform_id,omitempty"`
		Chassis    string `json:"chassis_id,omitempty"`
		Cluster    string `json:"cluster_id,omitempty"`
		Hypervisor string `json:"hypervisor_id,omitempty"`
		Blade      string `json:"blade_id,omitempty"`
		Node       string `json:"node_id,omitempty"`
		ZoneID     string `json:"zone_id,omitempty"`
	} `json:"location,omitempty"`

	IPV6 *ScalewayIPV6Definition `json:"ipv6,omitempty"`

	EnableIPV6 bool `json:"enable_ipv6,omitempty"`

	// This fields are not returned by the API, we generate it
	DNSPublic  string `json:"dns_public,omitempty"`
	DNSPrivate string `json:"dns_private,omitempty"`
}

// ScalewayServerPatchDefinition represents a Scaleway server with nullable fields (for PATCH)
type ScalewayServerPatchDefinition struct {
	Arch              *string                    `json:"arch,omitempty"`
	Name              *string                    `json:"name,omitempty"`
	CreationDate      *string                    `json:"creation_date,omitempty"`
	ModificationDate  *string                    `json:"modification_date,omitempty"`
	Image             *ScalewayImage             `json:"image,omitempty"`
	DynamicIPRequired *bool                      `json:"dynamic_ip_required,omitempty"`
	PublicAddress     *ScalewayIPAddress         `json:"public_ip,omitempty"`
	State             *string                    `json:"state,omitempty"`
	StateDetail       *string                    `json:"state_detail,omitempty"`
	PrivateIP         *string                    `json:"private_ip,omitempty"`
	Bootscript        *string                    `json:"bootscript,omitempty"`
	Hostname          *string                    `json:"hostname,omitempty"`
	Volumes           *map[string]ScalewayVolume `json:"volumes,omitempty"`
	SecurityGroup     *ScalewaySecurityGroup     `json:"security_group,omitempty"`
	Organization      *string                    `json:"organization,omitempty"`
	Tags              *[]string                  `json:"tags,omitempty"`
	IPV6              *ScalewayIPV6Definition    `json:"ipv6,omitempty"`
	EnableIPV6        *bool                      `json:"enable_ipv6,omitempty"`
}

// ScalewayServerDefinition represents a Scaleway server with image definition
type ScalewayServerDefinition struct {
	// Name is the user-defined name of the server
	Name string `json:"name"`

	// Image is the image used by the server
	Image *string `json:"image,omitempty"`

	// Volumes are the attached volumes
	Volumes map[string]string `json:"volumes,omitempty"`

	// DynamicIPRequired is a flag that defines a server with a dynamic ip address attached
	DynamicIPRequired *bool `json:"dynamic_ip_required,omitempty"`

	// Bootscript is the bootscript used by the server
	Bootscript *string `json:"bootscript"`

	// Tags are the metadata tags attached to the server
	Tags []string `json:"tags,omitempty"`

	// Organization is the owner of the server
	Organization string `json:"organization"`

	// CommercialType is the commercial type of the server (i.e: C1, C2[SML], VC1S)
	CommercialType string `json:"commercial_type"`

	PublicIP string `json:"public_ip,omitempty"`

	EnableIPV6 bool `json:"enable_ipv6,omitempty"`

	SecurityGroup string `json:"security_group,omitempty"`
}

// ScalewayServers represents a group of Scaleway servers
type ScalewayServers struct {
	// Servers holds scaleway servers of the response
	Servers []ScalewayServer `json:"servers,omitempty"`
}

// ScalewayServerAction represents an action to perform on a Scaleway server
type ScalewayServerAction struct {
	// Action is the name of the action to trigger
	Action string `json:"action,omitempty"`
}

// ScalewayOneServer represents the response of a GET /servers/UUID API call
type ScalewayOneServer struct {
	Server ScalewayServer `json:"server,omitempty"`
}

// PatchServer updates a server
func (s *ScalewayAPI) PatchServer(serverID string, definition ScalewayServerPatchDefinition) error {
	resp, err := s.PatchResponse(s.computeAPI, fmt.Sprintf("servers/%s", serverID), definition)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := s.handleHTTPError([]int{http.StatusOK}, resp); err != nil {
		return err
	}
	return nil
}

// GetServers gets the list of servers from the ScalewayAPI
func (s *ScalewayAPI) GetServers(all bool, limit int) (*[]ScalewayServer, error) {
	query := url.Values{}
	if !all {
		query.Set("state", "running")
	}
	if limit > 0 {
		// FIXME: wait for the API to be ready
		// query.Set("per_page", strconv.Itoa(limit))
		panic("Not implemented yet")
	}

	var (
		g    errgroup.Group
		apis = []string{
			ComputeAPIPar1,
			ComputeAPIAms1,
		}
	)

	serverChan := make(chan ScalewayServers, 2)
	for _, api := range apis {
		g.Go(s.fetchServers(api, query, serverChan))
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(serverChan)
	var servers ScalewayServers

	for server := range serverChan {
		servers.Servers = append(servers.Servers, server.Servers...)
	}

	for i, server := range servers.Servers {
		servers.Servers[i].DNSPublic = server.Identifier + URLPublicDNS
		servers.Servers[i].DNSPrivate = server.Identifier + URLPrivateDNS
	}
	return &servers.Servers, nil
}

// ScalewaySortServers represents a wrapper to sort by CreationDate the servers
type ScalewaySortServers []ScalewayServer

func (s ScalewaySortServers) Len() int {
	return len(s)
}

func (s ScalewaySortServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ScalewaySortServers) Less(i, j int) bool {
	date1, _ := time.Parse("2006-01-02T15:04:05.000000+00:00", s[i].CreationDate)
	date2, _ := time.Parse("2006-01-02T15:04:05.000000+00:00", s[j].CreationDate)
	return date2.Before(date1)
}

// GetServer gets a server from the ScalewayAPI
func (s *ScalewayAPI) GetServer(serverID string) (*ScalewayServer, error) {
	if serverID == "" {
		return nil, fmt.Errorf("cannot get server without serverID")
	}
	resp, err := s.GetResponsePaginate(s.computeAPI, "servers/"+serverID, url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}

	var oneServer ScalewayOneServer

	if err = json.Unmarshal(body, &oneServer); err != nil {
		return nil, err
	}
	// FIXME arch, owner, title
	oneServer.Server.DNSPublic = oneServer.Server.Identifier + URLPublicDNS
	oneServer.Server.DNSPrivate = oneServer.Server.Identifier + URLPrivateDNS
	return &oneServer.Server, nil
}

// PostServerAction posts an action on a server
func (s *ScalewayAPI) PostServerAction(serverID, action string) error {
	data := ScalewayServerAction{
		Action: action,
	}
	resp, err := s.PostResponse(s.computeAPI, fmt.Sprintf("servers/%s/action", serverID), data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusAccepted}, resp)
	return err
}

func (s *ScalewayAPI) fetchServers(api string, query url.Values, out chan<- ScalewayServers) func() error {
	return func() error {
		resp, err := s.GetResponsePaginate(api, "servers", query)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
		if err != nil {
			return err
		}
		var servers ScalewayServers

		if err = json.Unmarshal(body, &servers); err != nil {
			return err
		}
		out <- servers
		return nil
	}
}

// DeleteServer deletes a server
func (s *ScalewayAPI) DeleteServer(serverID string) error {
	resp, err := s.DeleteResponse(s.computeAPI, fmt.Sprintf("servers/%s", serverID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err = s.handleHTTPError([]int{http.StatusNoContent}, resp); err != nil {
		return err
	}
	return nil
}

// PostServer creates a new server
func (s *ScalewayAPI) PostServer(definition ScalewayServerDefinition) (string, error) {
	definition.Organization = s.Organization

	resp, err := s.PostResponse(s.computeAPI, "servers", definition)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusCreated}, resp)
	if err != nil {
		return "", err
	}
	var server ScalewayOneServer

	if err = json.Unmarshal(body, &server); err != nil {
		return "", err
	}
	return server.Server.Identifier, nil
}
