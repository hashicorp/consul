package policies

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// Policy is a firewall policy.
type Policy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TenantID    string   `json:"tenant_id"`
	ProjectID   string   `json:"project_id"`
	Audited     bool     `json:"audited"`
	Shared      bool     `json:"shared"`
	Rules       []string `json:"firewall_rules,omitempty"`
}

type commonResult struct {
	gophercloud.Result
}

// Extract is a function that accepts a result and extracts a firewall policy.
func (r commonResult) Extract() (*Policy, error) {
	var s struct {
		Policy *Policy `json:"firewall_policy"`
	}
	err := r.ExtractInto(&s)
	return s.Policy, err
}

// PolicyPage is the page returned by a pager when traversing over a
// collection of firewall policies.
type PolicyPage struct {
	pagination.LinkedPageBase
}

// NextPageURL is invoked when a paginated collection of firewall policies has
// reached the end of a page and the pager seeks to traverse over a new one.
// In order to do this, it needs to construct the next page's URL.
func (r PolicyPage) NextPageURL() (string, error) {
	var s struct {
		Links []gophercloud.Link `json:"firewall_policies_links"`
	}
	err := r.ExtractInto(&s)
	if err != nil {
		return "", err
	}
	return gophercloud.ExtractNextURL(s.Links)
}

// IsEmpty checks whether a PolicyPage struct is empty.
func (r PolicyPage) IsEmpty() (bool, error) {
	is, err := ExtractPolicies(r)
	return len(is) == 0, err
}

// ExtractPolicies accepts a Page struct, specifically a Policy struct,
// and extracts the elements into a slice of Policy structs. In other words,
// a generic collection is mapped into a relevant slice.
func ExtractPolicies(r pagination.Page) ([]Policy, error) {
	var s struct {
		Policies []Policy `json:"firewall_policies"`
	}
	err := (r.(PolicyPage)).ExtractInto(&s)
	return s.Policies, err
}

// GetResult represents the result of a get operation. Call its Extract
// method to interpret it as a Policy.
type GetResult struct {
	commonResult
}

// UpdateResult represents the result of an update operation. Call its
// Extract method to interpret it as a Policy.
type UpdateResult struct {
	commonResult
}

// DeleteResult represents the result of a delete operation. Call its
// ExtractErr method to determine if the operation succeeded or failed.
type DeleteResult struct {
	gophercloud.ErrResult
}

// CreateResult represents the result of a create operation. Call its Extract
// method to interpret it as a Policy.
type CreateResult struct {
	commonResult
}

// InsertRuleResult represents the result of an InsertRule operation. Call its
// Extract method to interpret it as a Policy.
type InsertRuleResult struct {
	commonResult
}

// RemoveRuleResult represents the result of a RemoveRule operation. Call its
// Extract method to interpret it as a Policy.
type RemoveRuleResult struct {
	commonResult
}
