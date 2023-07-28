package fsm

import (
	"fmt"
	"net"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/hashicorp/raft"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func init() {
	cePersister = persistCE
	registerRestorer(structs.RegisterRequestType, restoreRegistration)
	registerRestorer(structs.KVSRequestType, restoreKV)
	registerRestorer(structs.TombstoneRequestType, restoreTombstone)
	registerRestorer(structs.SessionRequestType, restoreSession)
	registerRestorer(structs.CoordinateBatchUpdateType, restoreCoordinates)
	registerRestorer(structs.PreparedQueryRequestType, restorePreparedQuery)
	registerRestorer(structs.AutopilotRequestType, restoreAutopilot)
	registerRestorer(structs.IntentionRequestType, restoreLegacyIntention)
	registerRestorer(structs.ConnectCARequestType, restoreConnectCA)
	registerRestorer(structs.ConnectCAProviderStateType, restoreConnectCAProviderState)
	registerRestorer(structs.ConnectCAConfigType, restoreConnectCAConfig)
	registerRestorer(structs.IndexRequestType, restoreIndex)
	registerRestorer(structs.ACLTokenSetRequestType, restoreToken)
	registerRestorer(structs.ACLPolicySetRequestType, restorePolicy)
	registerRestorer(structs.ConfigEntryRequestType, restoreConfigEntry)
	registerRestorer(structs.ACLRoleSetRequestType, restoreRole)
	registerRestorer(structs.ACLBindingRuleSetRequestType, restoreBindingRule)
	registerRestorer(structs.ACLAuthMethodSetRequestType, restoreAuthMethod)
	registerRestorer(structs.FederationStateRequestType, restoreFederationState)
	registerRestorer(structs.SystemMetadataRequestType, restoreSystemMetadata)
	registerRestorer(structs.ServiceVirtualIPRequestType, restoreServiceVirtualIP)
	registerRestorer(structs.FreeVirtualIPRequestType, restoreFreeVirtualIP)
	registerRestorer(structs.PeeringWriteType, restorePeering)
	registerRestorer(structs.PeeringTrustBundleWriteType, restorePeeringTrustBundle)
	registerRestorer(structs.PeeringSecretsWriteType, restorePeeringSecrets)
}

func persistCE(s *snapshot, sink raft.SnapshotSink, encoder *codec.Encoder) error {
	if err := s.persistVirtualIPs(sink, encoder); err != nil {
		return err
	}
	if err := s.persistNodes(sink, encoder); err != nil {
		return err
	}
	if err := s.persistSessions(sink, encoder); err != nil {
		return err
	}
	if err := s.persistACLs(sink, encoder); err != nil {
		return err
	}
	if err := s.persistKVs(sink, encoder); err != nil {
		return err
	}
	if err := s.persistTombstones(sink, encoder); err != nil {
		return err
	}
	if err := s.persistPreparedQueries(sink, encoder); err != nil {
		return err
	}
	if err := s.persistAutopilot(sink, encoder); err != nil {
		return err
	}
	if err := s.persistLegacyIntentions(sink, encoder); err != nil {
		return err
	}
	if err := s.persistConnectCA(sink, encoder); err != nil {
		return err
	}
	if err := s.persistConnectCAProviderState(sink, encoder); err != nil {
		return err
	}
	if err := s.persistConnectCAConfig(sink, encoder); err != nil {
		return err
	}
	if err := s.persistConfigEntries(sink, encoder); err != nil {
		return err
	}
	if err := s.persistFederationStates(sink, encoder); err != nil {
		return err
	}
	if err := s.persistSystemMetadata(sink, encoder); err != nil {
		return err
	}
	if err := s.persistIndex(sink, encoder); err != nil {
		return err
	}
	if err := s.persistPeerings(sink, encoder); err != nil {
		return err
	}
	if err := s.persistPeeringTrustBundles(sink, encoder); err != nil {
		return err
	}
	if err := s.persistPeeringSecrets(sink, encoder); err != nil {
		return err
	}
	return nil
}

func (s *snapshot) persistNodes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the nodes
	nodes, err := s.state.Nodes()
	if err != nil {
		return err
	}

	// Register each node
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		nodeEntMeta := n.GetEnterpriseMeta()

		req := n.ToRegisterRequest()

		// Register the node itself
		if _, err := sink.Write([]byte{byte(structs.RegisterRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(&req); err != nil {
			return err
		}

		// Register each service this node has
		services, err := s.state.Services(n.Node, nodeEntMeta, n.PeerName)
		if err != nil {
			return err
		}
		for service := services.Next(); service != nil; service = services.Next() {
			if _, err := sink.Write([]byte{byte(structs.RegisterRequestType)}); err != nil {
				return err
			}
			req.Service = service.(*structs.ServiceNode).ToNodeService()
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}

		// Register each check this node has
		req.Service = nil
		checks, err := s.state.Checks(n.Node, nodeEntMeta, n.PeerName)
		if err != nil {
			return err
		}
		for check := checks.Next(); check != nil; check = checks.Next() {
			if _, err := sink.Write([]byte{byte(structs.RegisterRequestType)}); err != nil {
				return err
			}
			req.Check = check.(*structs.HealthCheck)
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}
	}

	// Save the coordinates separately since they are not part of the
	// register request interface. To avoid copying them out, we turn
	// them into batches with a single coordinate each.
	coords, err := s.state.Coordinates()
	if err != nil {
		return err
	}
	for coord := coords.Next(); coord != nil; coord = coords.Next() {
		if _, err := sink.Write([]byte{byte(structs.CoordinateBatchUpdateType)}); err != nil {
			return err
		}
		updates := structs.Coordinates{coord.(*structs.Coordinate)}
		if err := encoder.Encode(&updates); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistSessions(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	sessions, err := s.state.Sessions()
	if err != nil {
		return err
	}

	for session := sessions.Next(); session != nil; session = sessions.Next() {
		if _, err := sink.Write([]byte{byte(structs.SessionRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(session.(*structs.Session)); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistACLs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	tokens, err := s.state.ACLTokens()
	if err != nil {
		return err
	}

	// Don't check expiration times. Wait for explicit deletions.

	for token := tokens.Next(); token != nil; token = tokens.Next() {
		if _, err := sink.Write([]byte{byte(structs.ACLTokenSetRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(token.(*structs.ACLToken)); err != nil {
			return err
		}
	}

	policies, err := s.state.ACLPolicies()
	if err != nil {
		return err
	}

	for policy := policies.Next(); policy != nil; policy = policies.Next() {
		if _, err := sink.Write([]byte{byte(structs.ACLPolicySetRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(policy.(*structs.ACLPolicy)); err != nil {
			return err
		}
	}

	roles, err := s.state.ACLRoles()
	if err != nil {
		return err
	}

	for role := roles.Next(); role != nil; role = roles.Next() {
		if _, err := sink.Write([]byte{byte(structs.ACLRoleSetRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(role.(*structs.ACLRole)); err != nil {
			return err
		}
	}

	rules, err := s.state.ACLBindingRules()
	if err != nil {
		return err
	}

	for rule := rules.Next(); rule != nil; rule = rules.Next() {
		if _, err := sink.Write([]byte{byte(structs.ACLBindingRuleSetRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(rule.(*structs.ACLBindingRule)); err != nil {
			return err
		}
	}

	methods, err := s.state.ACLAuthMethods()
	if err != nil {
		return err
	}

	for method := methods.Next(); method != nil; method = methods.Next() {
		if _, err := sink.Write([]byte{byte(structs.ACLAuthMethodSetRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(method.(*structs.ACLAuthMethod)); err != nil {
			return err
		}
	}

	return nil
}

func (s *snapshot) persistKVs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	entries, err := s.state.KVs()
	if err != nil {
		return err
	}

	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		if _, err := sink.Write([]byte{byte(structs.KVSRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(*structs.DirEntry)); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistTombstones(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	stones, err := s.state.Tombstones()
	if err != nil {
		return err
	}

	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		if _, err := sink.Write([]byte{byte(structs.TombstoneRequestType)}); err != nil {
			return err
		}

		// For historical reasons, these are serialized in the snapshots
		// as KV entries. We want to keep the snapshot format compatible
		// with pre-0.6 versions for now.
		s := stone.(*state.Tombstone)
		fake := &structs.DirEntry{
			Key: s.Key,
			RaftIndex: structs.RaftIndex{
				ModifyIndex: s.Index,
			},
		}
		if err := encoder.Encode(fake); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistPreparedQueries(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	queries, err := s.state.PreparedQueries()
	if err != nil {
		return err
	}

	for _, query := range queries {
		if _, err := sink.Write([]byte{byte(structs.PreparedQueryRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(query); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistAutopilot(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	config, err := s.state.Autopilot()
	if err != nil {
		return err
	}
	// Make sure we don't write a nil config out to a snapshot.
	if config == nil {
		return nil
	}

	if _, err := sink.Write([]byte{byte(structs.AutopilotRequestType)}); err != nil {
		return err
	}
	if err := encoder.Encode(config); err != nil {
		return err
	}
	return nil
}

func (s *snapshot) persistConnectCA(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	roots, err := s.state.CARoots()
	if err != nil {
		return err
	}

	for _, r := range roots {
		if _, err := sink.Write([]byte{byte(structs.ConnectCARequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(r); err != nil {
			return err
		}
	}

	return nil
}

func (s *snapshot) persistConnectCAConfig(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	config, err := s.state.CAConfig()
	if err != nil {
		return err
	}
	// Make sure we don't write a nil config out to a snapshot.
	if config == nil {
		return nil
	}

	if _, err := sink.Write([]byte{byte(structs.ConnectCAConfigType)}); err != nil {
		return err
	}
	if err := encoder.Encode(config); err != nil {
		return err
	}
	return nil
}

func (s *snapshot) persistConnectCAProviderState(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	state, err := s.state.CAProviderState()
	if err != nil {
		return err
	}

	for _, r := range state {
		if _, err := sink.Write([]byte{byte(structs.ConnectCAProviderStateType)}); err != nil {
			return err
		}
		if err := encoder.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistLegacyIntentions(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	//nolint:staticcheck
	ixns, err := s.state.LegacyIntentions()
	if err != nil {
		return err
	}

	for _, ixn := range ixns {
		if _, err := sink.Write([]byte{byte(structs.IntentionRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(ixn); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistConfigEntries(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	entries, err := s.state.ConfigEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err := sink.Write([]byte{byte(structs.ConfigEntryRequestType)}); err != nil {
			return err
		}
		// Encode the entry request without an operation since we don't need it for restoring.
		// The request is used for its custom decoding/encoding logic around the ConfigEntry
		// interface.
		req := &structs.ConfigEntryRequest{
			Entry: entry,
		}
		if err := encoder.Encode(req); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistFederationStates(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	fedStates, err := s.state.FederationStates()
	if err != nil {
		return err
	}

	for _, fedState := range fedStates {
		if _, err := sink.Write([]byte{byte(structs.FederationStateRequestType)}); err != nil {
			return err
		}
		// Encode the entry request without an operation since we don't need it for restoring.
		// The request is used for its custom decoding/encoding logic around the ConfigEntry
		// interface.
		req := &structs.FederationStateRequest{
			Op:    structs.FederationStateUpsert,
			State: fedState,
		}
		if err := encoder.Encode(req); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistSystemMetadata(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	entries, err := s.state.SystemMetadataEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err := sink.Write([]byte{byte(structs.SystemMetadataRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistIndex(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	// Get all the indexes
	iter, err := s.state.Indexes()
	if err != nil {
		return err
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		// Prepare the request struct
		idx := raw.(*state.IndexEntry)

		// Write out a node registration
		sink.Write([]byte{byte(structs.IndexRequestType)})
		if err := encoder.Encode(idx); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persistVirtualIPs(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	serviceVIPs, err := s.state.ServiceVirtualIPs()
	if err != nil {
		return err
	}

	for entry := serviceVIPs.Next(); entry != nil; entry = serviceVIPs.Next() {
		if _, err := sink.Write([]byte{byte(structs.ServiceVirtualIPRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(state.ServiceVirtualIP)); err != nil {
			return err
		}
	}

	freeVIPs, err := s.state.FreeVirtualIPs()
	if err != nil {
		return err
	}

	for entry := freeVIPs.Next(); entry != nil; entry = freeVIPs.Next() {
		if _, err := sink.Write([]byte{byte(structs.FreeVirtualIPRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(state.FreeVirtualIP)); err != nil {
			return err
		}
	}

	return nil
}

func (s *snapshot) persistPeerings(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	peerings, err := s.state.Peerings()
	if err != nil {
		return err
	}

	for entry := peerings.Next(); entry != nil; entry = peerings.Next() {
		if _, err := sink.Write([]byte{byte(structs.PeeringWriteType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(*pbpeering.Peering)); err != nil {
			return err
		}
	}

	return nil
}

func (s *snapshot) persistPeeringTrustBundles(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	ptbs, err := s.state.PeeringTrustBundles()
	if err != nil {
		return err
	}

	for entry := ptbs.Next(); entry != nil; entry = ptbs.Next() {
		if _, err := sink.Write([]byte{byte(structs.PeeringTrustBundleWriteType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(*pbpeering.PeeringTrustBundle)); err != nil {
			return err
		}
	}

	return nil
}

func (s *snapshot) persistPeeringSecrets(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	secrets, err := s.state.PeeringSecrets()
	if err != nil {
		return err
	}

	for entry := secrets.Next(); entry != nil; entry = secrets.Next() {
		if _, err := sink.Write([]byte{byte(structs.PeeringSecretsWriteType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(*pbpeering.PeeringSecrets)); err != nil {
			return err
		}
	}

	return nil
}

func restoreRegistration(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.RegisterRequest
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.Registration(header.LastIndex, &req); err != nil {
		return err
	}
	return nil
}

func restoreKV(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.DirEntry
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.KVS(&req); err != nil {
		return err
	}
	return nil
}

func restoreTombstone(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.DirEntry
	if err := decoder.Decode(&req); err != nil {
		return err
	}

	// For historical reasons, these are serialized in the
	// snapshots as KV entries. We want to keep the snapshot
	// format compatible with pre-0.6 versions for now.
	stone := &state.Tombstone{
		Key:   req.Key,
		Index: req.ModifyIndex,
	}
	if err := restore.Tombstone(stone); err != nil {
		return err
	}
	return nil
}

func restoreSession(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.Session
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.Session(&req); err != nil {
		return err
	}
	return nil
}

func restoreCoordinates(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.Coordinates
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.Coordinates(header.LastIndex, req); err != nil {
		return err
	}
	return nil
}

func restorePreparedQuery(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.PreparedQuery
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.PreparedQuery(&req); err != nil {
		return err
	}
	return nil
}

func restoreAutopilot(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.AutopilotConfig
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.Autopilot(&req); err != nil {
		return err
	}
	return nil
}

func restoreLegacyIntention(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.Intention
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	//nolint:staticcheck
	if err := restore.LegacyIntention(&req); err != nil {
		return err
	}
	return nil
}

func restoreConnectCA(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.CARoot
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.CARoot(&req); err != nil {
		return err
	}
	return nil
}

func restoreConnectCAProviderState(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.CAConsulProviderState
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.CAProviderState(&req); err != nil {
		return err
	}
	return nil
}

func restoreConnectCAConfig(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.CAConfiguration
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.CAConfig(&req); err != nil {
		return err
	}
	return nil
}

func restoreIndex(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req state.IndexEntry
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.IndexRestore(&req)
}

func restoreToken(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.ACLToken
	if err := decoder.Decode(&req); err != nil {
		return err
	}

	// only set if unset - mitigates a bug where converted legacy tokens could end up without a hash
	req.SetHash(false)

	return restore.ACLToken(&req)
}

func restorePolicy(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.ACLPolicy
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.ACLPolicy(&req)
}

func restoreConfigEntry(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.ConfigEntryRequest
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.ConfigEntry(req.Entry)
}

func restoreRole(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.ACLRole
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.ACLRole(&req)
}

func restoreBindingRule(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.ACLBindingRule
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.ACLBindingRule(&req)
}

func restoreAuthMethod(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.ACLAuthMethod
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.ACLAuthMethod(&req)
}

func restoreFederationState(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.FederationStateRequest
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.FederationState(req.State)
}

func restoreSystemMetadata(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req structs.SystemMetadataEntry
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	return restore.SystemMetadataEntry(&req)
}

func restoreServiceVirtualIP(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	// state.ServiceVirtualIP was changed in a breaking way in 1.13.0 (2e4cb6f77d2be36b02e9be0b289b24e5b0afb794).
	// We attempt to reconcile the older type by decoding to a map then decoding that map into
	// structs.PeeredServiceName first, and then structs.ServiceName.
	var req struct {
		Service map[string]interface{}
		IP      net.IP

		structs.RaftIndex
	}
	if err := decoder.Decode(&req); err != nil {
		return err
	}

	vip := state.ServiceVirtualIP{
		IP:        req.IP,
		RaftIndex: req.RaftIndex,
	}

	// PeeredServiceName is the expected primary key type.
	var psn structs.PeeredServiceName
	if err := mapstructure.Decode(req.Service, &psn); err != nil {
		return fmt.Errorf("cannot decode to structs.PeeredServiceName: %w", err)
	}
	vip.Service = psn

	// If the expected primary key field is empty, it must be the older ServiceName type.
	if vip.Service.ServiceName.Name == "" {
		var sn structs.ServiceName
		if err := mapstructure.Decode(req.Service, &sn); err != nil {
			return fmt.Errorf("cannot decode to structs.ServiceName: %w", err)
		}
		vip.Service = structs.PeeredServiceName{
			ServiceName: sn,
		}
	}

	if err := restore.ServiceVirtualIP(vip); err != nil {
		return err
	}
	return nil
}

func restoreFreeVirtualIP(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req state.FreeVirtualIP
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.FreeVirtualIP(req); err != nil {
		return err
	}
	return nil
}

func restorePeering(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req pbpeering.Peering
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.Peering(&req); err != nil {
		return err
	}
	return nil
}

func restorePeeringTrustBundle(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req pbpeering.PeeringTrustBundle
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.PeeringTrustBundle(&req); err != nil {
		return err
	}
	return nil
}

func restorePeeringSecrets(header *SnapshotHeader, restore *state.Restore, decoder *codec.Decoder) error {
	var req pbpeering.PeeringSecrets
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := restore.PeeringSecrets(&req); err != nil {
		return err
	}
	return nil
}
