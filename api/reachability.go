package api

import (
	"encoding/json"
	"time"
)

// TODO: docs
type ReachabilityOpts struct {
	// WAN is whether to show members from the WAN.
	WAN bool

	// Segment is the LAN segment to show members for. Setting this to the
	// AllSegments value above will show members in all segments.
	Segment string
}

// TODO: docs
func (op *Operator) ReachabilityProbe(opts ReachabilityOpts, q *WriteOptions) (*ReachabilityResponses, *WriteMeta, error) {
	r := op.c.newRequest("PUT", "/v1/operator/reachability")
	r.setWriteOptions(q)
	r.params.Set("segment", opts.Segment)
	if opts.WAN {
		r.params.Set("wan", "1")
	}

	rtt, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	wm := &WriteMeta{RequestTime: rtt}
	var out ReachabilityResponses
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}
	return &out, wm, nil
	// _, resp, err := requireOK(a.c.doRequest(r))
	// if err != nil {
	// 	return nil, err
	// }
	// defer resp.Body.Close()

	// var out []*AgentMember
	// if err := decodeBody(resp, &out); err != nil {
	// 	return nil, err
	// }
	// return out, nil
}

type ReachabilityResponses struct {
	Responses []*ReachabilityResponse
}

type ReachabilityResponse struct {
	// Whether this response is for the WAN
	WAN bool `json:",omitempty"`
	// The datacenter name this request corresponds to
	Datacenter string `json:",omitempty"`
	// Segment has the network segment this request corresponds to.
	Segment string `json:",omitempty"` // for LAN

	Error string `json:",omitempty"`

	NumNodes    int
	LiveMembers []string

	Acks []string // include duplicates

	// 	total := float64(time.Now().Sub(start)) / float64(time.Second)
	QueryTime time.Duration
	// 	timeToLast := float64(last.Sub(start)) / float64(time.Second)
	TimeToLastResponse time.Duration
}

func (r *ReachabilityResponse) UnmarshalJSON(data []byte) error {
	type Alias ReachabilityResponse
	aux := &struct {
		QueryTime          string
		TimeToLastResponse string
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.QueryTime != "" {
		if r.QueryTime, err = time.ParseDuration(aux.QueryTime); err != nil {
			return err
		}
	}
	if aux.TimeToLastResponse != "" {
		if r.TimeToLastResponse, err = time.ParseDuration(aux.TimeToLastResponse); err != nil {
			return err
		}
	}
	return nil
}
