package structs

import (
	"encoding/json"
	"time"

	"github.com/hashicorp/consul/lib"
)

type ReachabilityResponse struct {
	// Node is the name of the node conducting the test.
	Node string

	// Datacenter is the datacenter name that this response describes.
	Datacenter string

	// WAN indicates if this response was from the WAN serf pool.
	WAN bool `json:",omitempty"`

	// Segment has the network segment this response describes.
	Segment string `json:",omitempty"`

	NumNodes    int
	LiveMembers []string

	// Acks contains the node name of each ACK received. This contains
	// duplicates if duplicates were received.
	Acks []string

	QueryTimeout       time.Duration
	QueryTime          time.Duration
	TimeToLastResponse time.Duration
}

func (r *ReachabilityResponse) MarshalJSON() ([]byte, error) {
	type Alias ReachabilityResponse
	exported := &struct {
		QueryTimeout       string `json:",omitempty"`
		QueryTime          string `json:",omitempty"`
		TimeToLastResponse string `json:",omitempty"`
		*Alias
	}{
		QueryTimeout:       r.QueryTimeout.String(),
		QueryTime:          r.QueryTime.String(),
		TimeToLastResponse: r.TimeToLastResponse.String(),
		Alias:              (*Alias)(r),
	}
	if r.QueryTimeout == 0 {
		exported.QueryTimeout = ""
	}
	if r.QueryTime == 0 {
		exported.QueryTime = ""
	}
	if r.TimeToLastResponse == 0 {
		exported.TimeToLastResponse = ""
	}

	return json.Marshal(exported)
}

func (r *ReachabilityResponse) UnmarshalJSON(data []byte) error {
	type Alias ReachabilityResponse
	aux := &struct {
		QueryTimeout       string
		QueryTime          string
		TimeToLastResponse string
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.QueryTimeout != "" {
		if r.QueryTimeout, err = time.ParseDuration(aux.QueryTimeout); err != nil {
			return err
		}
	}
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
