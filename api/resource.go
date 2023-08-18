// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Resource struct {
	c *Client
}

type GVK struct {
	Group   string
	Version string
	Kind    string
}

type WriteRequest struct {
	Metadata map[string]string `json:"metadata"`
	Data     map[string]string `json:"data"`
	Owner    *pbresource.ID    `json:"owner"`
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

func (resource *Resource) Apply(gvk *GVK, resourceName string, q *QueryOptions, payload *WriteRequest) (map[string]interface{}, *WriteMeta, error) {
	url := strings.ToLower(fmt.Sprintf("/api/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, resourceName))

	r := resource.c.newRequest("PUT", url)
	r.setQueryOptions(q)
	r.obj = payload
	rtt, resp, err := resource.c.doRequest(r)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	wm := &WriteMeta{}
	wm.RequestTime = rtt

	var out map[string]interface{}
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}

	return out, wm, nil
}
