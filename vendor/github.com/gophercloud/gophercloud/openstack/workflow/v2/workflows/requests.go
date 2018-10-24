package workflows

import (
	"io"

	"github.com/gophercloud/gophercloud"
)

// CreateOptsBuilder allows extension to add additional parameters to the Create request.
type CreateOptsBuilder interface {
	ToWorkflowCreateParams() (io.Reader, string, error)
}

// CreateOpts specifies parameters used to create a cron trigger.
type CreateOpts struct {
	// Scope is the scope of the workflow.
	// Allowed values are "private" and "public".
	Scope string `q:"scope"`

	// Namespace will define the namespace of the workflow.
	Namespace string `q:"namespace"`

	// Definition is the workflow definition written in Mistral Workflow Language v2.
	Definition io.Reader
}

// ToWorkflowCreateParams constructs a request query string from CreateOpts.
func (opts CreateOpts) ToWorkflowCreateParams() (io.Reader, string, error) {
	q, err := gophercloud.BuildQueryString(opts)
	return opts.Definition, q.String(), err
}

// Create requests the creation of a new execution.
func Create(client *gophercloud.ServiceClient, opts CreateOptsBuilder) (r CreateResult) {
	url := createURL(client)
	var b io.Reader
	if opts != nil {
		tmpB, query, err := opts.ToWorkflowCreateParams()
		if err != nil {
			r.Err = err
			return
		}
		url += query
		b = tmpB
	}

	_, r.Err = client.Post(url, nil, &r.Body, &gophercloud.RequestOpts{
		RawBody: b,
		MoreHeaders: map[string]string{
			"Content-Type": "text/plain",
			"Accept":       "", // Drop default JSON Accept header
		},
	})

	return
}
