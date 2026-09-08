// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"net/url"
	"strconv"
)

type FeatureGate struct {
	Name                 string
	Stage                string
	MinVersion           string
	DefaultEnabled       bool
	BeforeMinimumVersion string
	Description          string
	Owner                string
	DesiredEnabled       bool
	EffectiveEnabled     bool
	Eligible             bool
	Source               string
	Reason               string
	PolicyIndex          uint64
	StatusIndex          uint64
}

type FeatureGateSetRequest struct {
	Enabled bool
}

type FeatureGateSetResponse struct {
	Applied bool
	Feature FeatureGate
}

func (op *Operator) FeatureGateList(q *QueryOptions) ([]FeatureGate, *QueryMeta, error) {
	r := op.c.newRequest("GET", "/v1/operator/features")
	r.setQueryOptions(q)
	rtt, resp, err := op.c.doRequest(r)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	meta := &QueryMeta{}
	parseQueryMeta(resp, meta)
	meta.RequestTime = rtt
	var out []FeatureGate
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}
	return out, meta, nil
}

func (op *Operator) FeatureGateGet(name string, q *QueryOptions) (*FeatureGate, *QueryMeta, error) {
	r := op.c.newRequest("GET", "/v1/operator/feature/"+url.PathEscape(name))
	r.setQueryOptions(q)
	rtt, resp, err := op.c.doRequest(r)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	meta := &QueryMeta{}
	parseQueryMeta(resp, meta)
	meta.RequestTime = rtt
	var out FeatureGate
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}
	return &out, meta, nil
}

// FeatureGateSet records explicit operator intent. expectedPolicyIndex zero
// means no caller-supplied CAS; the server still uses internal CAS fencing.
func (op *Operator) FeatureGateSet(name string, enabled bool, expectedPolicyIndex uint64, q *WriteOptions) (*FeatureGateSetResponse, error) {
	r := op.c.newRequest("PUT", "/v1/operator/feature/"+url.PathEscape(name))
	r.setWriteOptions(q)
	if expectedPolicyIndex != 0 {
		r.params.Set("cas", strconv.FormatUint(expectedPolicyIndex, 10))
	}
	r.obj = FeatureGateSetRequest{Enabled: enabled}
	_, resp, err := op.c.doRequest(r)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, err
	}

	var out FeatureGateSetResponse
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
