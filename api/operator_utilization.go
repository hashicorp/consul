package api

import (
	"io"
	"strconv"
)

type Census struct {
	client *Client
}

// UtilizationBundleRequest configures generation of a census utilization bundle.
type UtilizationBundleRequest struct {
	CustomerID string
	Message    string
	TodayOnly  bool
	SendReport bool
}

func (c *Client) Census() *Census {
	return &Census{c}
}

// UtilizationBundle generates a census utilization bundle by calling the
// /v1/operator/utilization endpoint. The returned byte slice contains the bundle
// JSON payload suitable for saving to disk or further processing.
func (c *Census) UtilizationBundle(req *UtilizationBundleRequest, q *QueryOptions) ([]byte, *QueryMeta, error) {
	r := c.client.newRequest("GET", "/v1/operator/utilization")
	if q != nil {
		r.setQueryOptions(q)
	}
	if req != nil {
		if req.CustomerID != "" {
			r.params.Set("customer_id", req.CustomerID)
		}
		if req.Message != "" {
			r.params.Set("message", req.Message)
		}
		if req.TodayOnly {
			r.params.Set("today_only", strconv.FormatBool(true))
		}
		if req.SendReport {
			r.params.Set("send_report", strconv.FormatBool(true))
		}
	}

	rtt, resp, err := c.client.doRequest(r)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, qm, nil
}
