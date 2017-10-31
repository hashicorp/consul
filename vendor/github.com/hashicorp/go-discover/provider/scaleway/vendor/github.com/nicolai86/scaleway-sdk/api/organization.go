package api

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// ScalewayOrganizationDefinition represents a Scaleway Organization
type ScalewayOrganizationDefinition struct {
	ID    string                   `json:"id"`
	Name  string                   `json:"name"`
	Users []ScalewayUserDefinition `json:"users"`
}

// ScalewayOrganizationsDefinition represents a Scaleway Organizations
type ScalewayOrganizationsDefinition struct {
	Organizations []ScalewayOrganizationDefinition `json:"organizations"`
}

// GetOrganization returns Organization
func (s *ScalewayAPI) GetOrganization() (*ScalewayOrganizationsDefinition, error) {
	resp, err := s.GetResponsePaginate(AccountAPI, "organizations", url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var data ScalewayOrganizationsDefinition

	if err = json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
