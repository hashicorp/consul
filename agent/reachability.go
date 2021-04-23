package agent

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func (a *Agent) CheckReachability(wan bool, segmentName string, timeout time.Duration) ([]*structs.ReachabilityResponse, error) {
	if wan {
		return a.checkReachabilityWAN(timeout)
	}

	switch segmentName {
	case "", api.AllSegments:
		return a.checkReachabilityAllLANSegments(timeout)
	default:
		return a.checkReachabilityLANSegment(segmentName, timeout)
	}
}

func (a *Agent) checkReachabilityWAN(timeout time.Duration) ([]*structs.ReachabilityResponse, error) {
	srv, ok := a.delegate.(*consul.Server)
	if !ok {
		return nil, errors.New("Must be a server to execute a WAN reachability test")
	}

	pool := srv.WANPool()

	resp, err := a.checkSegmentReachability(pool, timeout)
	if err != nil {
		return nil, err
	}
	resp.WAN = true

	return []*structs.ReachabilityResponse{resp}, nil
}

func (a *Agent) checkReachabilityAllLANSegments(timeout time.Duration) ([]*structs.ReachabilityResponse, error) {
	segments := a.delegate.LANSegments()

	var (
		out  []*structs.ReachabilityResponse
		errs error
	)
	for name, segment := range segments {
		resp, err := a.checkSegmentReachability(segment, timeout)
		if err != nil {
			if name == "" {
				name = "<default>"
			}
			err = fmt.Errorf("error sending reachability query to segment %q: %v", name, err)
			errs = multierror.Append(errs, err)
		}
		resp.Segment = name
		out = append(out, resp)
	}
	if errs != nil {
		return nil, errs
	}
	return out, nil
}

func (a *Agent) checkReachabilityLANSegment(segmentName string, timeout time.Duration) ([]*structs.ReachabilityResponse, error) {
	segments := a.delegate.LANSegments()

	segment, ok := segments[segmentName]
	if !ok {
		return nil, fmt.Errorf("unknown segment %q", segmentName)
	}

	resp, err := a.checkSegmentReachability(segment, timeout)
	if err != nil {
		return nil, err
	}
	resp.Segment = segmentName

	return []*structs.ReachabilityResponse{resp}, nil
}

func (a *Agent) checkSegmentReachability(segment *serf.Serf, timeout time.Duration) (*structs.ReachabilityResponse, error) {
	members := segment.Members()

	// Get only the live members
	var (
		liveMembers      = make(map[string]struct{})
		liveMembersSlice []string
	)
	for _, m := range members {
		if m.Status == serf.StatusAlive {
			if _, ok := liveMembers[m.Name]; !ok {
				liveMembers[m.Name] = struct{}{}
				liveMembersSlice = append(liveMembersSlice, m.Name)
			}
		}
	}
	sort.Strings(liveMembersSlice)

	if timeout == 0 {
		timeout = segment.DefaultQueryTimeout()
	}

	resp, err := segment.Query(
		serf.InternalQueryPrefix+"ping",
		[]byte{},
		&serf.QueryParam{
			RequestAck: true,
			Timeout:    timeout,
		},
	)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	start := time.Now()
	last := start

	// Track responses and acknowledgements
	acks := make([]string, 0, len(members))
	for !resp.Finished() {
		a := <-resp.AckCh()
		if a == "" {
			break
		}
		acks = append(acks, a)
		last = time.Now()
	}

	return &structs.ReachabilityResponse{
		Node:               segment.LocalMember().Name,
		Datacenter:         a.config.Datacenter,
		NumNodes:           len(members),
		LiveMembers:        liveMembersSlice,
		Acks:               acks,
		QueryTimeout:       timeout,
		QueryTime:          time.Now().Sub(start),
		TimeToLastResponse: last.Sub(start),
	}, nil
}
