// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package consul

import (
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/reporting"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

// runEnterpriseRateLimiterConfigEntryController start the rate limiter config controller
func (s *Server) runEnterpriseRateLimiterConfigEntryController() error {
	return nil
}

func (s *Server) registerEnterpriseGRPCServices(deps Deps, srv *grpc.Server) {}

func (s *Server) enterpriseValidateJoinWAN() error {
	return nil // no-op
}

// JoinLAN is used to have Consul join the inner-DC pool The target address
// should be another node inside the DC listening on the Serf LAN address
func (s *Server) JoinLAN(addrs []string, entMeta *acl.EnterpriseMeta) (int, error) {
	return s.serfLAN.Join(addrs, true)
}

// removeFailedNode is used to remove a failed node from the cluster
//
// if node is empty, just remove wanNode from the WAN
func (s *Server) removeFailedNode(
	removeFn func(*serf.Serf, string) error,
	node, wanNode string,
	entMeta *acl.EnterpriseMeta,
) error {
	maybeRemove := func(s *serf.Serf, node string) (bool, error) {
		if !isSerfMember(s, node) {
			return false, nil
		}
		return true, removeFn(s, node)
	}

	foundAny := false

	var merr error

	if node != "" {
		if found, err := maybeRemove(s.serfLAN, node); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("could not remove failed node from LAN: %w", err))
		} else if found {
			foundAny = true
		}
	}

	if s.serfWAN != nil {
		if found, err := maybeRemove(s.serfWAN, wanNode); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("could not remove failed node from WAN: %w", err))
		} else if found {
			foundAny = true
		}
	}

	if merr != nil {
		return merr
	}

	if !foundAny {
		return fmt.Errorf("agent: No node found with name '%s'", node)
	}

	return nil
}

// lanPoolAllMembers only returns our own segment or partition's members, because
// CE servers can't be in multiple segments or partitions.
func (s *Server) lanPoolAllMembers() ([]serf.Member, error) {
	return s.LANMembersInAgentPartition(), nil
}

// LANMembers returns the LAN members for one of:
//
// - the requested partition
// - the requested segment
// - all segments
//
// This is limited to segments and partitions that the node is a member of.
func (s *Server) LANMembers(filter LANMemberFilter) ([]serf.Member, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if filter.Segment != "" {
		return nil, structs.ErrSegmentsNotSupported
	}
	return s.LANMembersInAgentPartition(), nil
}

func (s *Server) GetMatchingLANCoordinate(_, _ string) (*coordinate.Coordinate, error) {
	return s.serfLAN.GetCoordinate()
}

func (s *Server) addEnterpriseLANCoordinates(cs lib.CoordinateSet) error {
	return nil
}

func (s *Server) LANSendUserEvent(name string, payload []byte, coalesce bool) error {
	err := s.serfLAN.UserEvent(name, payload, coalesce)
	if err != nil {
		return fmt.Errorf("error broadcasting event: %w", err)
	}
	return nil
}

func (s *Server) DoWithLANSerfs(
	fn func(name, poolKind string, pool *serf.Serf) error,
	errorFn func(name, poolKind string, err error) error,
) error {
	if errorFn == nil {
		errorFn = func(_, _ string, err error) error { return err }
	}
	err := fn("", "", s.serfLAN)
	if err != nil {
		return errorFn("", "", err)
	}
	return nil
}

// reconcile is used to reconcile the differences between Serf membership and
// what is reflected in our strongly consistent store. Mainly we need to ensure
// all live nodes are registered, all failed nodes are marked as such, and all
// left nodes are deregistered.
func (s *Server) reconcile() (err error) {
	defer metrics.MeasureSince([]string{"leader", "reconcile"}, time.Now())

	members := s.serfLAN.Members()
	knownMembers := make(map[string]struct{})
	for _, member := range members {
		memberName := strings.ToLower(member.Name)
		if err := s.reconcileMember(member); err != nil {
			return err
		}
		knownMembers[memberName] = struct{}{}
	}

	// Reconcile any members that have been reaped while we were not the
	// leader.
	return s.reconcileReaped(knownMembers, nil)
}

func (s *Server) addEnterpriseStats(stats map[string]map[string]string) {
	// no-op
}

func getSerfMemberEnterpriseMeta(member serf.Member) *acl.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}

func addSerfMetricsLabels(conf *serf.Config, wan bool, segment string, partition string, areaID string) {
	conf.MetricLabels = []metrics.Label{}

	networkMetric := metrics.Label{
		Name: "network",
	}
	if wan {
		networkMetric.Value = "wan"
	} else {
		networkMetric.Value = "lan"
	}

	conf.MetricLabels = append(conf.MetricLabels, networkMetric)
}

func (s *Server) updateReportingConfig(config ReloadableConfig) {
	// no-op
}

func getEnterpriseReportingDeps(deps Deps) reporting.EntDeps {
	// no-op
	return reporting.EntDeps{}
}
