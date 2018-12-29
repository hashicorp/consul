package workflows

import (
	"encoding/json"
	"time"

	"github.com/gophercloud/gophercloud"
)

// CreateResult is the response of a Post operations. Call its Extract method to interpret it as a list of Workflows.
type CreateResult struct {
	gophercloud.Result
}

// Extract helps to get created Workflow struct from a Create function.
func (r CreateResult) Extract() ([]Workflow, error) {
	var s struct {
		Workflows []Workflow `json:"workflows"`
	}
	err := r.ExtractInto(&s)
	return s.Workflows, err
}

// Workflow represents a workflow execution on OpenStack mistral API.
type Workflow struct {
	// ID is the workflow's unique ID.
	ID string `json:"id"`

	// Definition is the workflow definition in Mistral v2 DSL.
	Definition string `json:"definition"`

	// Name is the name of the workflow.
	Name string `json:"name"`

	// Namespace is the namespace of the workflow.
	Namespace string `json:"namespace"`

	// Input represents the needed input to execute the workflow.
	// This parameter is a list of each input, comma separated.
	Input string `json:"input"`

	// ProjectID is the project id owner of the workflow.
	ProjectID string `json:"project_id"`

	// Scope is the scope of the workflow.
	// Values can be "private" or "public".
	Scope string `json:"scope"`

	// Tags is a list of tags associated to the workflow.
	Tags []string `json:"tags"`

	// CreatedAt is the creation date of the workflow.
	CreatedAt time.Time `json:"-"`

	// UpdatedAt is the last update date of the workflow.
	UpdatedAt *time.Time `json:"-"`
}

// UnmarshalJSON implements unmarshalling custom types
func (r *Workflow) UnmarshalJSON(b []byte) error {
	type tmp Workflow
	var s struct {
		tmp
		CreatedAt gophercloud.JSONRFC3339ZNoTNoZ  `json:"created_at"`
		UpdatedAt *gophercloud.JSONRFC3339ZNoTNoZ `json:"updated_at"`
	}

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	*r = Workflow(s.tmp)

	r.CreatedAt = time.Time(s.CreatedAt)
	if s.UpdatedAt != nil {
		t := time.Time(*s.UpdatedAt)
		r.UpdatedAt = &t
	}

	return nil
}
