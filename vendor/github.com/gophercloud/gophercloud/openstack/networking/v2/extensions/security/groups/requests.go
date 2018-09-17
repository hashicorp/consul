package groups

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// ListOpts allows the filtering and sorting of paginated collections through
// the API. Filtering is achieved by passing in struct field values that map to
// the group attributes you want to see returned. SortKey allows you to
// sort by a particular network attribute. SortDir sets the direction, and is
// either `asc' or `desc'. Marker and Limit are used for pagination.
type ListOpts struct {
	ID        string `q:"id"`
	Name      string `q:"name"`
	TenantID  string `q:"tenant_id"`
	ProjectID string `q:"project_id"`
	Limit     int    `q:"limit"`
	Marker    string `q:"marker"`
	SortKey   string `q:"sort_key"`
	SortDir   string `q:"sort_dir"`
}

// List returns a Pager which allows you to iterate over a collection of
// security groups. It accepts a ListOpts struct, which allows you to filter
// and sort the returned collection for greater efficiency.
func List(c *gophercloud.ServiceClient, opts ListOpts) pagination.Pager {
	q, err := gophercloud.BuildQueryString(&opts)
	if err != nil {
		return pagination.Pager{Err: err}
	}
	u := rootURL(c) + q.String()
	return pagination.NewPager(c, u, func(r pagination.PageResult) pagination.Page {
		return SecGroupPage{pagination.LinkedPageBase{PageResult: r}}
	})
}

// CreateOptsBuilder allows extensions to add additional parameters to the
// Create request.
type CreateOptsBuilder interface {
	ToSecGroupCreateMap() (map[string]interface{}, error)
}

// CreateOpts contains all the values needed to create a new security group.
type CreateOpts struct {
	// Human-readable name for the Security Group. Does not have to be unique.
	Name string `json:"name" required:"true"`

	// TenantID is the UUID of the project who owns the Group.
	// Only administrative users can specify a tenant UUID other than their own.
	TenantID string `json:"tenant_id,omitempty"`

	// ProjectID is the UUID of the project who owns the Group.
	// Only administrative users can specify a tenant UUID other than their own.
	ProjectID string `json:"project_id,omitempty"`

	// Describes the security group.
	Description string `json:"description,omitempty"`
}

// ToSecGroupCreateMap builds a request body from CreateOpts.
func (opts CreateOpts) ToSecGroupCreateMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "security_group")
}

// Create is an operation which provisions a new security group with default
// security group rules for the IPv4 and IPv6 ether types.
func Create(c *gophercloud.ServiceClient, opts CreateOptsBuilder) (r CreateResult) {
	b, err := opts.ToSecGroupCreateMap()
	if err != nil {
		r.Err = err
		return
	}
	_, r.Err = c.Post(rootURL(c), b, &r.Body, nil)
	return
}

// UpdateOptsBuilder allows extensions to add additional parameters to the
// Update request.
type UpdateOptsBuilder interface {
	ToSecGroupUpdateMap() (map[string]interface{}, error)
}

// UpdateOpts contains all the values needed to update an existing security
// group.
type UpdateOpts struct {
	// Human-readable name for the Security Group. Does not have to be unique.
	Name string `json:"name,omitempty"`

	// Describes the security group.
	Description string `json:"description,omitempty"`
}

// ToSecGroupUpdateMap builds a request body from UpdateOpts.
func (opts UpdateOpts) ToSecGroupUpdateMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "security_group")
}

// Update is an operation which updates an existing security group.
func Update(c *gophercloud.ServiceClient, id string, opts UpdateOptsBuilder) (r UpdateResult) {
	b, err := opts.ToSecGroupUpdateMap()
	if err != nil {
		r.Err = err
		return
	}

	_, r.Err = c.Put(resourceURL(c, id), b, &r.Body, &gophercloud.RequestOpts{
		OkCodes: []int{200},
	})
	return
}

// Get retrieves a particular security group based on its unique ID.
func Get(c *gophercloud.ServiceClient, id string) (r GetResult) {
	_, r.Err = c.Get(resourceURL(c, id), &r.Body, nil)
	return
}

// Delete will permanently delete a particular security group based on its
// unique ID.
func Delete(c *gophercloud.ServiceClient, id string) (r DeleteResult) {
	_, r.Err = c.Delete(resourceURL(c, id), nil)
	return
}

// IDFromName is a convenience function that returns a security group's ID,
// given its name.
func IDFromName(client *gophercloud.ServiceClient, name string) (string, error) {
	count := 0
	id := ""

	listOpts := ListOpts{
		Name: name,
	}

	pages, err := List(client, listOpts).AllPages()
	if err != nil {
		return "", err
	}

	all, err := ExtractGroups(pages)
	if err != nil {
		return "", err
	}

	for _, s := range all {
		if s.Name == name {
			count++
			id = s.ID
		}
	}

	switch count {
	case 0:
		return "", gophercloud.ErrResourceNotFound{Name: name, ResourceType: "security group"}
	case 1:
		return id, nil
	default:
		return "", gophercloud.ErrMultipleResourcesFound{Name: name, Count: count, ResourceType: "security group"}
	}
}
