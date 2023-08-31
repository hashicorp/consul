// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"fmt"
	"strings"
)

type Resource struct {
	c *Client
}

type GVK struct {
	Group   string
	Version string
	Kind    string
}

// Config returns a handle to the Config endpoints
func (c *Client) Resource() *Resource {
	return &Resource{c}
}

func (resource *Resource) Read(gvk *GVK, resourceName string, q *QueryOptions) (map[string]interface{}, error) {
	r := resource.c.newRequest("GET", strings.ToLower(fmt.Sprintf("/api/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, resourceName)))
	r.setQueryOptions(q)
	_, resp, err := resource.c.doRequest(r)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, err
	}

	var out map[string]interface{}
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}

	return out, nil
}
