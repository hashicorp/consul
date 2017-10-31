package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewayQuota represents a map of quota (name, value)
type ScalewayQuota map[string]int

// ScalewayGetQuotas represents the response of GET /organizations/{orga_id}/quotas
type ScalewayGetQuotas struct {
	Quotas ScalewayQuota `json:"quotas"`
}

// GetQuotas returns a ScalewayGetQuotas
func (s *ScalewayAPI) GetQuotas() (*ScalewayGetQuotas, error) {
	resp, err := s.GetResponsePaginate(AccountAPI, fmt.Sprintf("organizations/%s/quotas", s.Organization), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var quotas ScalewayGetQuotas

	if err = json.Unmarshal(body, &quotas); err != nil {
		return nil, err
	}
	return &quotas, nil
}
