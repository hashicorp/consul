package clusters

import (
	"net/http"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

// CreateOptsBuilder Builder.
type CreateOptsBuilder interface {
	ToClusterCreateMap() (map[string]interface{}, error)
}

// CreateOpts params
type CreateOpts struct {
	ClusterTemplateID string            `json:"cluster_template_id" required:"true"`
	CreateTimeout     *int              `json:"create_timeout"`
	DiscoveryURL      string            `json:"discovery_url,omitempty"`
	DockerVolumeSize  *int              `json:"docker_volume_size,omitempty"`
	FlavorID          string            `json:"flavor_id,omitempty"`
	Keypair           string            `json:"keypair,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	MasterCount       *int              `json:"master_count,omitempty"`
	MasterFlavorID    string            `json:"master_flavor_id,omitempty"`
	Name              string            `json:"name"`
	NodeCount         *int              `json:"node_count,omitempty"`
}

// ToClusterCreateMap constructs a request body from CreateOpts.
func (opts CreateOpts) ToClusterCreateMap() (map[string]interface{}, error) {
	return gophercloud.BuildRequestBody(opts, "")
}

// Create requests the creation of a new cluster.
func Create(client *gophercloud.ServiceClient, opts CreateOptsBuilder) (r CreateResult) {
	b, err := opts.ToClusterCreateMap()
	if err != nil {
		r.Err = err
		return
	}
	var result *http.Response
	result, r.Err = client.Post(createURL(client), b, &r.Body, &gophercloud.RequestOpts{
		OkCodes: []int{202},
	})

	if r.Err == nil {
		r.Header = result.Header
	}

	return
}

// Get retrieves a specific clusters based on its unique ID.
func Get(client *gophercloud.ServiceClient, id string) (r GetResult) {
	var result *http.Response
	result, r.Err = client.Get(getURL(client, id), &r.Body, &gophercloud.RequestOpts{OkCodes: []int{200}})
	if r.Err == nil {
		r.Header = result.Header
	}
	return
}

// ListOptsBuilder allows extensions to add additional parameters to the
// List request.
type ListOptsBuilder interface {
	ToClustersListQuery() (string, error)
}

// ListOpts allows the sorting of paginated collections through
// the API. SortKey allows you to sort by a particular cluster attribute.
// SortDir sets the direction, and is either `asc' or `desc'.
// Marker and Limit are used for pagination.
type ListOpts struct {
	Marker  string `q:"marker"`
	Limit   int    `q:"limit"`
	SortKey string `q:"sort_key"`
	SortDir string `q:"sort_dir"`
}

// ToClustersListQuery formats a ListOpts into a query string.
func (opts ListOpts) ToClustersListQuery() (string, error) {
	q, err := gophercloud.BuildQueryString(opts)
	return q.String(), err
}

// List returns a Pager which allows you to iterate over a collection of
// clusters. It accepts a ListOptsBuilder, which allows you to sort
// the returned collection for greater efficiency.
func List(c *gophercloud.ServiceClient, opts ListOptsBuilder) pagination.Pager {
	url := listURL(c)
	if opts != nil {
		query, err := opts.ToClustersListQuery()
		if err != nil {
			return pagination.Pager{Err: err}
		}
		url += query
	}
	return pagination.NewPager(c, url, func(r pagination.PageResult) pagination.Page {
		return ClusterPage{pagination.LinkedPageBase{PageResult: r}}
	})
}
