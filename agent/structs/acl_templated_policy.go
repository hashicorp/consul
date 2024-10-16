// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"bytes"
	_ "embed"
	"fmt"
	"hash"
	"hash/fnv"
	"text/template"

	"github.com/xeipuuv/gojsonschema"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib/stringslice"
)

//go:embed acltemplatedpolicy/schemas/node.json
var ACLTemplatedPolicyNodeSchema string

//go:embed acltemplatedpolicy/schemas/service.json
var ACLTemplatedPolicyServiceSchema string

//go:embed acltemplatedpolicy/schemas/api-gateway.json
var ACLTemplatedPolicyAPIGatewaySchema string

type ACLTemplatedPolicies []*ACLTemplatedPolicy

const (
	ACLTemplatedPolicyServiceID     = "00000000-0000-0000-0000-000000000003"
	ACLTemplatedPolicyNodeID        = "00000000-0000-0000-0000-000000000004"
	ACLTemplatedPolicyDNSID         = "00000000-0000-0000-0000-000000000005"
	ACLTemplatedPolicyNomadServerID = "00000000-0000-0000-0000-000000000006"
	_                               = "00000000-0000-0000-0000-000000000007" // formerly workload identity
	ACLTemplatedPolicyAPIGatewayID  = "00000000-0000-0000-0000-000000000008"
	ACLTemplatedPolicyNomadClientID = "00000000-0000-0000-0000-000000000009"

	ACLTemplatedPolicyServiceDescription     = "Gives the token or role permissions to register a service and discover services in the Consul catalog. It also gives the specified service's sidecar proxy the permission to discover and route traffic to other services."
	ACLTemplatedPolicyNodeDescription        = "Gives the token or role permissions for a register an agent/node into the catalog. A node is typically a consul agent but can also be a physical server, cloud instance or a container."
	ACLTemplatedPolicyDNSDescription         = "Gives the token or role permissions for the Consul DNS to query services in the network."
	ACLTemplatedPolicyNomadServerDescription = "Gives the token or role permissions required for integration with a nomad server."
	ACLTemplatedPolicyAPIGatewayDescription  = "Gives the token or role permissions for a Consul api gateway"
	ACLTemplatedPolicyNomadClientDescription = "Gives the token or role permissions required for integration with a nomad client."

	ACLTemplatedPolicyNoRequiredVariablesSchema = "" // catch-all schema for all templated policy that don't require a schema
)

// ACLTemplatedPolicyBase contains basic information about builtin templated policies
// template name, id, template code and schema
type ACLTemplatedPolicyBase struct {
	TemplateName string
	TemplateID   string
	Schema       string
	Template     string
	Description  string
}

var (
	// Note: when adding a new builtin template, ensure you update `command/acl/templatedpolicy/formatter.go`
	// to handle the new templates required variables and schema.
	aclTemplatedPoliciesList = map[string]*ACLTemplatedPolicyBase{
		api.ACLTemplatedPolicyServiceName: {
			TemplateID:   ACLTemplatedPolicyServiceID,
			TemplateName: api.ACLTemplatedPolicyServiceName,
			Schema:       ACLTemplatedPolicyServiceSchema,
			Template:     ACLTemplatedPolicyService,
			Description:  ACLTemplatedPolicyServiceDescription,
		},
		api.ACLTemplatedPolicyNodeName: {
			TemplateID:   ACLTemplatedPolicyNodeID,
			TemplateName: api.ACLTemplatedPolicyNodeName,
			Schema:       ACLTemplatedPolicyNodeSchema,
			Template:     ACLTemplatedPolicyNode,
			Description:  ACLTemplatedPolicyNodeDescription,
		},
		api.ACLTemplatedPolicyDNSName: {
			TemplateID:   ACLTemplatedPolicyDNSID,
			TemplateName: api.ACLTemplatedPolicyDNSName,
			Schema:       ACLTemplatedPolicyNoRequiredVariablesSchema,
			Template:     ACLTemplatedPolicyDNS,
			Description:  ACLTemplatedPolicyDNSDescription,
		},
		api.ACLTemplatedPolicyNomadServerName: {
			TemplateID:   ACLTemplatedPolicyNomadServerID,
			TemplateName: api.ACLTemplatedPolicyNomadServerName,
			Schema:       ACLTemplatedPolicyNoRequiredVariablesSchema,
			Template:     ACLTemplatedPolicyNomadServer,
			Description:  ACLTemplatedPolicyNomadServerDescription,
		},
		api.ACLTemplatedPolicyAPIGatewayName: {
			TemplateID:   ACLTemplatedPolicyAPIGatewayID,
			TemplateName: api.ACLTemplatedPolicyAPIGatewayName,
			Schema:       ACLTemplatedPolicyAPIGatewaySchema,
			Template:     ACLTemplatedPolicyAPIGateway,
			Description:  ACLTemplatedPolicyAPIGatewayDescription,
		},
		api.ACLTemplatedPolicyNomadClientName: {
			TemplateID:   ACLTemplatedPolicyNomadClientID,
			TemplateName: api.ACLTemplatedPolicyNomadClientName,
			Schema:       ACLTemplatedPolicyNoRequiredVariablesSchema,
			Template:     ACLTemplatedPolicyNomadClient,
			Description:  ACLTemplatedPolicyNomadClientDescription,
		},
	}
)

// ACLTemplatedPolicy represents a template used to generate a `synthetic` policy
// given some input variables.
type ACLTemplatedPolicy struct {
	// TemplateID are hidden from all displays and should not be exposed to the users.
	TemplateID string `json:",omitempty"`

	// TemplateName is used for display purposes mostly and should not be used for policy rendering.
	TemplateName string `json:",omitempty"`

	// TemplateVariables are input variables required to render templated policies.
	TemplateVariables *ACLTemplatedPolicyVariables `json:",omitempty"`

	// Datacenters that the synthetic policy will be valid within.
	//   - No wildcards allowed
	//   - If empty then the synthetic policy is valid within all datacenters
	//
	// This is kept for legacy reasons to enable us to replace Node/Service Identities by templated policies.
	//
	// Only valid for global tokens. It is an error to specify this for local tokens.
	Datacenters []string `json:",omitempty"`
}

// ACLTemplatedPolicyVariables are input variables required to render templated policies.
type ACLTemplatedPolicyVariables struct {
	Name string `json:"name,omitempty"`
}

func (tp *ACLTemplatedPolicy) Clone() *ACLTemplatedPolicy {
	tp2 := *tp

	tp2.TemplateVariables = nil
	if tp.TemplateVariables != nil {
		tp2.TemplateVariables = tp.TemplateVariables.Clone()
	}
	tp2.Datacenters = stringslice.CloneStringSlice(tp.Datacenters)

	return &tp2
}

func (tp *ACLTemplatedPolicy) AddToHash(h hash.Hash) {
	h.Write([]byte(tp.TemplateID))
	h.Write([]byte(tp.TemplateName))

	if tp.TemplateVariables != nil {
		tp.TemplateVariables.AddToHash(h)
	}
	for _, dc := range tp.Datacenters {
		h.Write([]byte(dc))
	}
}

func (tv *ACLTemplatedPolicyVariables) AddToHash(h hash.Hash) {
	h.Write([]byte(tv.Name))
}

func (tv *ACLTemplatedPolicyVariables) Clone() *ACLTemplatedPolicyVariables {
	tv2 := *tv
	return &tv2
}

// validates templated policy variables against schema.
func (tp *ACLTemplatedPolicy) ValidateTemplatedPolicy(schema string) error {
	if schema == "" {
		return nil
	}

	loader := gojsonschema.NewStringLoader(schema)
	dataloader := gojsonschema.NewGoLoader(tp.TemplateVariables)
	res, err := gojsonschema.Validate(loader, dataloader)
	if err != nil {
		return fmt.Errorf("failed to load json schema for validation %w", err)
	}

	// validate service and node identity names
	if tp.TemplateVariables != nil {
		if tp.TemplateName == api.ACLTemplatedPolicyServiceName && !acl.IsValidServiceIdentityName(tp.TemplateVariables.Name) {
			return fmt.Errorf("service identity %q has an invalid name. Only lowercase alphanumeric characters, '-' and '_' are allowed", tp.TemplateVariables.Name)
		}

		if tp.TemplateName == api.ACLTemplatedPolicyNodeName && !acl.IsValidNodeIdentityName(tp.TemplateVariables.Name) {
			return fmt.Errorf("node identity %q  has an invalid name. Only lowercase alphanumeric characters, '-' and '_' are allowed", tp.TemplateVariables.Name)
		}
	}

	if res.Valid() {
		return nil
	}

	var merr *multierror.Error

	for _, resultError := range res.Errors() {
		merr = multierror.Append(merr, fmt.Errorf(resultError.Description()))
	}
	return merr.ErrorOrNil()
}

func (tp *ACLTemplatedPolicy) EstimateSize() int {
	size := len(tp.TemplateName) + len(tp.TemplateID) + tp.TemplateVariables.EstimateSize()
	for _, dc := range tp.Datacenters {
		size += len(dc)
	}

	return size
}

func (tv *ACLTemplatedPolicyVariables) EstimateSize() int {
	return len(tv.Name)
}

// SyntheticPolicy generates a policy based on templated policies' ID and variables
//
// Given that we validate this string name before persisting, we do not
// have to escape it before doing the following interpolation.
func (tp *ACLTemplatedPolicy) SyntheticPolicy(entMeta *acl.EnterpriseMeta) (*ACLPolicy, error) {
	rules, err := tp.aclTemplatedPolicyRules(entMeta)
	if err != nil {
		return nil, err
	}
	hasher := fnv.New128a()
	hashID := fmt.Sprintf("%x", hasher.Sum([]byte(rules)))

	policy := &ACLPolicy{
		Rules:       rules,
		ID:          hashID,
		Name:        fmt.Sprintf("synthetic-policy-%s", hashID),
		Datacenters: tp.Datacenters,
		Description: fmt.Sprintf("synthetic policy generated from templated policy: %s", tp.TemplateName),
	}
	policy.EnterpriseMeta.Merge(entMeta)
	policy.SetHash(true)

	return policy, nil
}

func (tp *ACLTemplatedPolicy) aclTemplatedPolicyRules(entMeta *acl.EnterpriseMeta) (string, error) {
	if entMeta == nil {
		entMeta = DefaultEnterpriseMetaInDefaultPartition()
	}
	entMeta.Normalize()

	tpl := template.New(tp.TemplateName)
	tmplCode, ok := aclTemplatedPoliciesList[tp.TemplateName]
	if !ok {
		return "", fmt.Errorf("acl templated policy does not exist: %s", tp.TemplateName)
	}

	parsedTpl, err := tpl.Parse(tmplCode.Template)
	if err != nil {
		return "", fmt.Errorf("an error occured when parsing template structs: %w", err)
	}
	var buf bytes.Buffer
	err = parsedTpl.Execute(&buf, struct {
		*ACLTemplatedPolicyVariables
		Namespace string
		Partition string
	}{
		Namespace:                   entMeta.NamespaceOrDefault(),
		Partition:                   entMeta.PartitionOrDefault(),
		ACLTemplatedPolicyVariables: tp.TemplateVariables,
	})
	if err != nil {
		return "", fmt.Errorf("an error occured when executing on templated policy variables: %w", err)
	}

	return buf.String(), nil
}

// Deduplicate returns a new list of templated policies without duplicates.
// compares values of template variables to ensure no duplicates
func (tps ACLTemplatedPolicies) Deduplicate() ACLTemplatedPolicies {
	list := make(map[string][]ACLTemplatedPolicyVariables)
	var out ACLTemplatedPolicies

	for _, tp := range tps {
		// checks if template name already in the unique list
		_, found := list[tp.TemplateName]
		if !found {
			list[tp.TemplateName] = make([]ACLTemplatedPolicyVariables, 0)
		}
		templateSchema := aclTemplatedPoliciesList[tp.TemplateName].Schema

		// if schema is empty, template does not require variables
		if templateSchema == "" {
			if !found {
				out = append(out, tp)
			}
			continue
		}

		if !slices.Contains(list[tp.TemplateName], *tp.TemplateVariables) {
			list[tp.TemplateName] = append(list[tp.TemplateName], *tp.TemplateVariables)
			out = append(out, tp)
		}
	}

	return out
}

func GetACLTemplatedPolicyBase(templateName string) (*ACLTemplatedPolicyBase, bool) {
	if orig, found := aclTemplatedPoliciesList[templateName]; found {
		copy := *orig
		return &copy, found
	}

	return nil, false
}

// GetACLTemplatedPolicyList returns a copy of the list of templated policies
func GetACLTemplatedPolicyList() map[string]*ACLTemplatedPolicyBase {
	m := make(map[string]*ACLTemplatedPolicyBase, len(aclTemplatedPoliciesList))
	for k, v := range aclTemplatedPoliciesList {
		m[k] = v
	}

	return m
}
