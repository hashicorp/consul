package structs

import (
	"encoding/json"
	"time"

	"github.com/hashicorp/consul/lib"
)

type ReachabilityRequest struct {
	Datacenter string
	WAN        bool
	Segment    string
	QueryOptions
}

func (r *ReachabilityRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ReachabilityResponses struct {
	Responses []*ReachabilityResponse
	QueryMeta
}

type ReachabilityResponse struct {
	WAN        bool   `json:",omitempty"`
	Datacenter string `json:",omitempty"` // for WAN
	Segment    string `json:",omitempty"` // for LAN

	Error string `json:",omitempty"`

	NumNodes    int
	LiveMembers []string

	Acks []string // include duplicates

	// 	total := float64(time.Now().Sub(start)) / float64(time.Second)
	QueryTime time.Duration
	// 	timeToLast := float64(last.Sub(start)) / float64(time.Second)
	TimeToLastResponse time.Duration
}

func (r *ReachabilityResponse) MarshalJSON() ([]byte, error) {
	type Alias ReachabilityResponse
	exported := &struct {
		QueryTime          string `json:",omitempty"`
		TimeToLastResponse string `json:",omitempty"`
		*Alias
	}{
		QueryTime:          r.QueryTime.String(),
		TimeToLastResponse: r.TimeToLastResponse.String(),
		Alias:              (*Alias)(r),
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
