package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewayPermissions represents the response of GET /permissions
type ScalewayPermissions map[string]ScalewayPermCategory

// ScalewayPermCategory represents ScalewayPermissions's fields
type ScalewayPermCategory map[string][]string

// ScalewayPermissionDefinition represents the permissions
type ScalewayPermissionDefinition struct {
	Permissions ScalewayPermissions `json:"permissions"`
}

// GetPermissions returns the permissions
func (s *ScalewayAPI) GetPermissions() (*ScalewayPermissionDefinition, error) {
	resp, err := s.GetResponsePaginate(AccountAPI, fmt.Sprintf("tokens/%s/permissions", s.Token), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var permissions ScalewayPermissionDefinition

	if err = json.Unmarshal(body, &permissions); err != nil {
		return nil, err
	}
	return &permissions, nil
}
