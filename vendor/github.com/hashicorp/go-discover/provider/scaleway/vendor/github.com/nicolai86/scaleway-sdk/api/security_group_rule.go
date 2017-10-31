package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewaySecurityGroupRule definition
type ScalewaySecurityGroupRule struct {
	Direction    string `json:"direction"`
	Protocol     string `json:"protocol"`
	IPRange      string `json:"ip_range"`
	DestPortFrom int    `json:"dest_port_from,omitempty"`
	Action       string `json:"action"`
	Position     int    `json:"position"`
	DestPortTo   string `json:"dest_port_to"`
	Editable     bool   `json:"editable"`
	ID           string `json:"id"`
}

// ScalewayGetSecurityGroupRules represents the response of a GET /security_group/{groupID}/rules
type ScalewayGetSecurityGroupRules struct {
	Rules []ScalewaySecurityGroupRule `json:"rules"`
}

// ScalewayGetSecurityGroupRule represents the response of a GET /security_group/{groupID}/rules/{ruleID}
type ScalewayGetSecurityGroupRule struct {
	Rules ScalewaySecurityGroupRule `json:"rule"`
}

// ScalewayNewSecurityGroupRule definition POST/PUT request /security_group/{groupID}
type ScalewayNewSecurityGroupRule struct {
	Action       string `json:"action"`
	Direction    string `json:"direction"`
	IPRange      string `json:"ip_range"`
	Protocol     string `json:"protocol"`
	DestPortFrom int    `json:"dest_port_from,omitempty"`
}

// GetSecurityGroupRules returns a ScalewaySecurityGroupRules
func (s *ScalewayAPI) GetSecurityGroupRules(groupID string) (*ScalewayGetSecurityGroupRules, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, fmt.Sprintf("security_groups/%s/rules", groupID), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var securityGroupRules ScalewayGetSecurityGroupRules

	if err = json.Unmarshal(body, &securityGroupRules); err != nil {
		return nil, err
	}
	return &securityGroupRules, nil
}

// GetASecurityGroupRule returns a ScalewaySecurityGroupRule
func (s *ScalewayAPI) GetASecurityGroupRule(groupID string, rulesID string) (*ScalewayGetSecurityGroupRule, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, fmt.Sprintf("security_groups/%s/rules/%s", groupID, rulesID), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var securityGroupRules ScalewayGetSecurityGroupRule

	if err = json.Unmarshal(body, &securityGroupRules); err != nil {
		return nil, err
	}
	return &securityGroupRules, nil
}

// PostSecurityGroupRule posts a rule on a server
func (s *ScalewayAPI) PostSecurityGroupRule(SecurityGroupID string, rules ScalewayNewSecurityGroupRule) error {
	resp, err := s.PostResponse(s.computeAPI, fmt.Sprintf("security_groups/%s/rules", SecurityGroupID), rules)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusCreated}, resp)
	return err
}

// PutSecurityGroupRule updates a SecurityGroupRule
func (s *ScalewayAPI) PutSecurityGroupRule(rules ScalewayNewSecurityGroupRule, securityGroupID, RuleID string) error {
	resp, err := s.PutResponse(s.computeAPI, fmt.Sprintf("security_groups/%s/rules/%s", securityGroupID, RuleID), rules)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusOK}, resp)
	return err
}

// DeleteSecurityGroupRule deletes a SecurityGroupRule
func (s *ScalewayAPI) DeleteSecurityGroupRule(SecurityGroupID, RuleID string) error {
	resp, err := s.DeleteResponse(s.computeAPI, fmt.Sprintf("security_groups/%s/rules/%s", SecurityGroupID, RuleID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}
