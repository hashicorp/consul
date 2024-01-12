// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/types"
)

const (
	consulWorkloadPrefix = "consul-server-"
	consulPortNameServer = "server"
)

var _ ConsulRegistrator = (*V2ConsulRegistrator)(nil)

var resourceCmpOptions = []cmp.Option{
	protocmp.IgnoreFields(&pbresource.Resource{}, "status", "generation", "version"),
	protocmp.IgnoreFields(&pbresource.ID{}, "uid"),
	protocmp.Transform(),
	// Stringify any type passed to the sorter so that we can reliably compare most values.
	cmpopts.SortSlices(func(a, b any) bool { return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b) }),
}

type V2ConsulRegistrator struct {
	Logger   hclog.Logger
	NodeName string
	EntMeta  *acl.EnterpriseMeta

	Client pbresource.ResourceServiceClient
}

// HandleAliveMember is used to ensure the server is registered as a Workload
// with a passing health check.
func (r V2ConsulRegistrator) HandleAliveMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta, joinServer func(m serf.Member, parts *metadata.Server) error) error {
	valid, parts := metadata.IsConsulServer(member)
	if !valid {
		return nil
	}

	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Attempt to join the consul server, regardless of the existing catalog state
	if err := joinServer(member, parts); err != nil {
		return err
	}

	r.Logger.Info("member joined, creating catalog entries",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

	workloadResource, err := r.createWorkloadFromMember(member, parts, nodeEntMeta)
	if err != nil {
		return err
	}

	// Check if the Workload already exists and if it's the same
	res, err := r.Client.Read(context.TODO(), &pbresource.ReadRequest{Id: workloadResource.Id})
	if err != nil && !grpcNotFoundErr(err) {
		return fmt.Errorf("error checking for existing Workload %s: %w", workloadResource.Id.Name, err)
	}

	if err == nil {
		existingWorkload := res.GetResource()

		r.Logger.Debug("existing Workload matching the member found",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)

		// If the Workload is identical, move to updating the health status
		if cmp.Equal(workloadResource, existingWorkload, resourceCmpOptions...) {
			r.Logger.Debug("no updates to perform on member Workload",
				"member", member.Name,
				"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
			)
			goto HEALTHSTATUS
		}

		// If the existing Workload different, add the existing Version into the patch for CAS write
		workloadResource.Id = existingWorkload.Id
		workloadResource.Version = existingWorkload.Version
	}

	if _, err := r.Client.Write(context.TODO(), &pbresource.WriteRequest{Resource: workloadResource}); err != nil {
		return fmt.Errorf("failed to write Workload %s: %w", workloadResource.Id.Name, err)
	}

	r.Logger.Info("updated consul Workload in catalog",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

HEALTHSTATUS:
	hsResource, err := r.createHealthStatusFromMember(member, workloadResource.Id, true, nodeEntMeta)
	if err != nil {
		return err
	}

	// Check if the HealthStatus already exists and if it's the same
	res, err = r.Client.Read(context.TODO(), &pbresource.ReadRequest{Id: hsResource.Id})
	if err != nil && !grpcNotFoundErr(err) {
		return fmt.Errorf("error checking for existing HealthStatus %s: %w", hsResource.Id.Name, err)
	}

	if err == nil {
		existingHS := res.GetResource()

		r.Logger.Debug("existing HealthStatus matching the member found",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)

		// If the HealthStatus is identical, we're done.
		if cmp.Equal(hsResource, existingHS, resourceCmpOptions...) {
			r.Logger.Debug("no updates to perform on member HealthStatus",
				"member", member.Name,
				"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
			)
			return nil
		}

		// If the existing HealthStatus is different, add the Version to the patch for CAS write.
		hsResource.Id = existingHS.Id
		hsResource.Version = existingHS.Version
	}

	if _, err := r.Client.Write(context.TODO(), &pbresource.WriteRequest{Resource: hsResource}); err != nil {
		return fmt.Errorf("failed to write HealthStatus %s: %w", hsResource.Id.Name, err)
	}
	r.Logger.Info("updated consul HealthStatus in catalog",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)
	return nil
}

func (r V2ConsulRegistrator) createWorkloadFromMember(member serf.Member, parts *metadata.Server, nodeEntMeta *acl.EnterpriseMeta) (*pbresource.Resource, error) {
	workloadMeta := map[string]string{
		"read_replica":          strconv.FormatBool(member.Tags["read_replica"] == "1"),
		"raft_version":          strconv.Itoa(parts.RaftVersion),
		"serf_protocol_current": strconv.FormatUint(uint64(member.ProtocolCur), 10),
		"serf_protocol_min":     strconv.FormatUint(uint64(member.ProtocolMin), 10),
		"serf_protocol_max":     strconv.FormatUint(uint64(member.ProtocolMax), 10),
		"version":               parts.Build.String(),
	}

	if parts.ExternalGRPCPort > 0 {
		workloadMeta["grpc_port"] = strconv.Itoa(parts.ExternalGRPCPort)
	}
	if parts.ExternalGRPCTLSPort > 0 {
		workloadMeta["grpc_tls_port"] = strconv.Itoa(parts.ExternalGRPCTLSPort)
	}

	workload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: member.Addr.String(), Ports: []string{consulPortNameServer}},
		},
		// Don't include identity since Consul is not routable through the mesh.
		// Don't include locality because these values are not passed along through serf, and they are probably
		// different from the leader's values.
		Ports: map[string]*pbcatalog.WorkloadPort{
			consulPortNameServer: {
				Port:     uint32(parts.Port),
				Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
			},
			// TODO: add other agent ports
		},
	}

	workloadData, err := anypb.New(workload)
	if err != nil {
		return nil, fmt.Errorf("could not convert Workload to 'any' type: %w", err)
	}

	workloadId := &pbresource.ID{
		Name:    fmt.Sprintf("%s%s", consulWorkloadPrefix, types.NodeID(member.Tags["id"])),
		Type:    pbcatalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	workloadId.Tenancy.Partition = nodeEntMeta.PartitionOrDefault()

	return &pbresource.Resource{
		Id:       workloadId,
		Data:     workloadData,
		Metadata: workloadMeta,
	}, nil
}

func (r V2ConsulRegistrator) createHealthStatusFromMember(member serf.Member, workloadId *pbresource.ID, passing bool, nodeEntMeta *acl.EnterpriseMeta) (*pbresource.Resource, error) {
	hs := &pbcatalog.HealthStatus{
		Type:        string(structs.SerfCheckID),
		Description: structs.SerfCheckName,
	}

	if passing {
		hs.Status = pbcatalog.Health_HEALTH_PASSING
		hs.Output = structs.SerfCheckAliveOutput
	} else {
		hs.Status = pbcatalog.Health_HEALTH_CRITICAL
		hs.Output = structs.SerfCheckFailedOutput
	}

	hsData, err := anypb.New(hs)
	if err != nil {
		return nil, fmt.Errorf("could not convert HealthStatus to 'any' type: %w", err)
	}

	hsId := &pbresource.ID{
		Name:    fmt.Sprintf("%s%s", consulWorkloadPrefix, types.NodeID(member.Tags["id"])),
		Type:    pbcatalog.HealthStatusType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	hsId.Tenancy.Partition = nodeEntMeta.PartitionOrDefault()

	return &pbresource.Resource{
		Id:    hsId,
		Data:  hsData,
		Owner: workloadId,
	}, nil
}

// HandleFailedMember is used to mark the workload's associated HealthStatus.
func (r V2ConsulRegistrator) HandleFailedMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta) error {
	if valid, _ := metadata.IsConsulServer(member); !valid {
		return nil
	}

	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	r.Logger.Info("member failed",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

	// Validate that the associated workload exists
	workloadId := &pbresource.ID{
		Name:    fmt.Sprintf("%s%s", consulWorkloadPrefix, types.NodeID(member.Tags["id"])),
		Type:    pbcatalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	workloadId.Tenancy.Partition = nodeEntMeta.PartitionOrDefault()

	res, err := r.Client.Read(context.TODO(), &pbresource.ReadRequest{Id: workloadId})
	if err != nil && !grpcNotFoundErr(err) {
		return fmt.Errorf("error checking for existing Workload %s: %w", workloadId.Name, err)
	}
	if grpcNotFoundErr(err) {
		r.Logger.Info("ignoring failed event for member because it does not exist in the catalog",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}
	// Overwrite the workload ID with the one that has UID populated.
	existingWorkload := res.GetResource()

	hsResource, err := r.createHealthStatusFromMember(member, existingWorkload.Id, false, nodeEntMeta)
	if err != nil {
		return err
	}

	res, err = r.Client.Read(context.TODO(), &pbresource.ReadRequest{Id: hsResource.Id})
	if err != nil && !grpcNotFoundErr(err) {
		return fmt.Errorf("error checking for existing HealthStatus %s: %w", hsResource.Id.Name, err)
	}

	if err == nil {
		existingHS := res.GetResource()
		r.Logger.Debug("existing HealthStatus matching the member found",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)

		// If the HealthStatus is identical, we're done.
		if cmp.Equal(hsResource, existingHS, resourceCmpOptions...) {
			r.Logger.Debug("no updates to perform on member HealthStatus",
				"member", member.Name,
				"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
			)
			return nil
		}

		// If the existing HealthStatus is different, add the Version to the patch for CAS write.
		hsResource.Id = existingHS.Id
		hsResource.Version = existingHS.Version
	}

	if _, err := r.Client.Write(context.TODO(), &pbresource.WriteRequest{Resource: hsResource}); err != nil {
		return fmt.Errorf("failed to write HealthStatus %s: %w", hsResource.Id.Name, err)
	}
	r.Logger.Info("updated consul HealthStatus in catalog",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)
	return nil
}

// HandleLeftMember is used to handle members that gracefully
// left. They are removed if necessary.
func (r V2ConsulRegistrator) HandleLeftMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta, removeServerFunc func(m serf.Member) error) error {
	return r.handleDeregisterMember("left", member, nodeEntMeta, removeServerFunc)
}

// HandleReapMember is used to handle members that have been
// reaped after a prolonged failure. They are removed from the catalog.
func (r V2ConsulRegistrator) HandleReapMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta, removeServerFunc func(m serf.Member) error) error {
	return r.handleDeregisterMember("reaped", member, nodeEntMeta, removeServerFunc)
}

// handleDeregisterMember is used to remove a member of a given reason
func (r V2ConsulRegistrator) handleDeregisterMember(reason string, member serf.Member, nodeEntMeta *acl.EnterpriseMeta, removeServerFunc func(m serf.Member) error) error {
	if valid, _ := metadata.IsConsulServer(member); !valid {
		return nil
	}

	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	r.Logger.Info("removing member",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		"reason", reason,
	)

	if err := removeServerFunc(member); err != nil {
		return err
	}

	// Do not remove our self. This can only happen if the current leader
	// is leaving. Instead, we should allow a follower to take-over and
	// remove us later.
	if strings.EqualFold(member.Name, r.NodeName) &&
		strings.EqualFold(nodeEntMeta.PartitionOrDefault(), r.EntMeta.PartitionOrDefault()) {
		r.Logger.Warn("removing self should be done by follower",
			"name", r.NodeName,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
			"reason", reason,
		)
		return nil
	}

	// Check if the workload exists
	workloadID := &pbresource.ID{
		Name:    fmt.Sprintf("%s%s", consulWorkloadPrefix, types.NodeID(member.Tags["id"])),
		Type:    pbcatalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	workloadID.Tenancy.Partition = nodeEntMeta.PartitionOrDefault()

	res, err := r.Client.Read(context.TODO(), &pbresource.ReadRequest{Id: workloadID})
	if err != nil && !grpcNotFoundErr(err) {
		return fmt.Errorf("error checking for existing Workload %s: %w", workloadID.Name, err)
	}
	if grpcNotFoundErr(err) {
		r.Logger.Info("ignoring reap event for member because it does not exist in the catalog",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}
	existingWorkload := res.GetResource()

	// The HealthStatus should be reaped automatically
	if _, err := r.Client.Delete(context.TODO(), &pbresource.DeleteRequest{Id: existingWorkload.Id}); err != nil {
		return fmt.Errorf("failed to delete Workload %s: %w", existingWorkload.Id.Name, err)
	}
	r.Logger.Info("deleted consul Workload",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)
	return err
}

func grpcNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.NotFound
}
