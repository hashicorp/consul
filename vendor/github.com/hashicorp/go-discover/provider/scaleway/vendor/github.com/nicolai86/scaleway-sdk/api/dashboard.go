package api

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// ScalewayDashboardResp represents a dashboard received from the API
type ScalewayDashboardResp struct {
	Dashboard ScalewayDashboard
}

// ScalewayDashboard represents a dashboard
type ScalewayDashboard struct {
	VolumesCount        int `json:"volumes_count"`
	RunningServersCount int `json:"running_servers_count"`
	ImagesCount         int `json:"images_count"`
	SnapshotsCount      int `json:"snapshots_count"`
	ServersCount        int `json:"servers_count"`
	IPsCount            int `json:"ips_count"`
}

// GetDashboard returns the dashboard
func (s *ScalewayAPI) GetDashboard() (*ScalewayDashboard, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "dashboard", url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var dashboard ScalewayDashboardResp

	if err = json.Unmarshal(body, &dashboard); err != nil {
		return nil, err
	}
	return &dashboard.Dashboard, nil
}
