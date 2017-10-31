package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewaySecurityGroups definition
type ScalewaySecurityGroups struct {
	Description           string                  `json:"description"`
	ID                    string                  `json:"id"`
	Organization          string                  `json:"organization"`
	Name                  string                  `json:"name"`
	Servers               []ScalewaySecurityGroup `json:"servers"`
	EnableDefaultSecurity bool                    `json:"enable_default_security"`
	OrganizationDefault   bool                    `json:"organization_default"`
}

// ScalewayGetSecurityGroups represents the response of a GET /security_groups/
type ScalewayGetSecurityGroups struct {
	SecurityGroups []ScalewaySecurityGroups `json:"security_groups"`
}

// ScalewayGetSecurityGroup represents the response of a GET /security_groups/{groupID}
type ScalewayGetSecurityGroup struct {
	SecurityGroups ScalewaySecurityGroups `json:"security_group"`
}

// ScalewaySecurityGroup represents a Scaleway security group
type ScalewaySecurityGroup struct {
	// Identifier is a unique identifier for the security group
	Identifier string `json:"id,omitempty"`

	// Name is the user-defined name of the security group
	Name string `json:"name,omitempty"`
}

// ScalewayNewSecurityGroup definition POST request /security_groups
type ScalewayNewSecurityGroup struct {
	Organization string `json:"organization"`
	Name         string `json:"name"`
	Description  string `json:"description"`
}

// ScalewayUpdateSecurityGroup definition PUT request /security_groups
type ScalewayUpdateSecurityGroup struct {
	Organization        string `json:"organization"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	OrganizationDefault bool   `json:"organization_default"`
}

// DeleteSecurityGroup deletes a SecurityGroup
func (s *ScalewayAPI) DeleteSecurityGroup(securityGroupID string) error {
	resp, err := s.DeleteResponse(s.computeAPI, fmt.Sprintf("security_groups/%s", securityGroupID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}

// PutSecurityGroup updates a SecurityGroup
func (s *ScalewayAPI) PutSecurityGroup(group ScalewayUpdateSecurityGroup, securityGroupID string) error {
	resp, err := s.PutResponse(s.computeAPI, fmt.Sprintf("security_groups/%s", securityGroupID), group)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusOK}, resp)
	return err
}

// GetASecurityGroup returns a ScalewaySecurityGroup
func (s *ScalewayAPI) GetASecurityGroup(groupsID string) (*ScalewayGetSecurityGroup, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, fmt.Sprintf("security_groups/%s", groupsID), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var securityGroups ScalewayGetSecurityGroup

	if err = json.Unmarshal(body, &securityGroups); err != nil {
		return nil, err
	}
	return &securityGroups, nil
}

// PostSecurityGroup posts a group on a server
func (s *ScalewayAPI) PostSecurityGroup(group ScalewayNewSecurityGroup) error {
	resp, err := s.PostResponse(s.computeAPI, "security_groups", group)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusCreated}, resp)
	return err
}

// GetSecurityGroups returns a ScalewaySecurityGroups
func (s *ScalewayAPI) GetSecurityGroups() (*ScalewayGetSecurityGroups, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "security_groups", url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var securityGroups ScalewayGetSecurityGroups

	if err = json.Unmarshal(body, &securityGroups); err != nil {
		return nil, err
	}
	return &securityGroups, nil
}
