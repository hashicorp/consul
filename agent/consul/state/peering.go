package state

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/maps"
	"github.com/hashicorp/consul/proto/pbpeering"
)

const (
	tablePeering             = "peering"
	tablePeeringTrustBundles = "peering-trust-bundles"
	tablePeeringSecrets      = "peering-secrets"
	tablePeeringSecretUUIDs  = "peering-secret-uuids"
)

func peeringTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tablePeering,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[string, *pbpeering.Peering]{
					readIndex:  indexFromUUIDString,
					writeIndex: indexIDFromPeering,
				},
			},
			indexName: {
				Name:         indexName,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[Query, *pbpeering.Peering, any]{
					readIndex:   indexPeeringFromQuery,
					writeIndex:  indexFromPeering,
					prefixIndex: prefixIndexFromQueryNoNamespace,
				},
			},
			indexDeleted: {
				Name:         indexDeleted,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[BoolQuery, *pbpeering.Peering]{
					readIndex:  indexDeletedFromBoolQuery,
					writeIndex: indexDeletedFromPeering,
				},
			},
		},
	}
}

func peeringTrustBundlesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tablePeeringTrustBundles,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[Query, *pbpeering.PeeringTrustBundle, any]{
					readIndex:   indexPeeringFromQuery, // same as peering table since we'll use the query.Value
					writeIndex:  indexFromPeeringTrustBundle,
					prefixIndex: prefixIndexFromQueryNoNamespace,
				},
			},
		},
	}
}

func peeringSecretsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tablePeeringSecrets,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[string, *pbpeering.PeeringSecrets]{
					readIndex:  indexFromUUIDString,
					writeIndex: indexIDFromPeeringSecret,
				},
			},
		},
	}
}

func peeringSecretUUIDsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tablePeeringSecretUUIDs,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[string, string]{
					readIndex:  indexFromUUIDString,
					writeIndex: indexFromUUIDString,
				},
			},
		},
	}
}

func indexIDFromPeeringSecret(p *pbpeering.PeeringSecrets) ([]byte, error) {
	if p.PeerID == "" {
		return nil, errMissingValueForIndex
	}

	uuid, err := uuidStringToBytes(p.PeerID)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func indexIDFromPeering(p *pbpeering.Peering) ([]byte, error) {
	if p.ID == "" {
		return nil, errMissingValueForIndex
	}

	uuid, err := uuidStringToBytes(p.ID)
	if err != nil {
		return nil, err
	}
	var b indexBuilder
	b.Raw(uuid)
	return b.Bytes(), nil
}

func indexDeletedFromPeering(p *pbpeering.Peering) ([]byte, error) {
	var b indexBuilder
	b.Bool(!p.IsActive())
	return b.Bytes(), nil
}

func (s *Store) PeeringSecretsRead(ws memdb.WatchSet, peerID string) (*pbpeering.PeeringSecrets, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	secret, err := peeringSecretsReadByPeerIDTxn(tx, ws, peerID)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		// TODO (peering) Return the tables index so caller can watch it for changes if the secret doesn't exist.
		return nil, nil
	}

	return secret, nil
}

func peeringSecretsReadByPeerIDTxn(tx ReadTxn, ws memdb.WatchSet, id string) (*pbpeering.PeeringSecrets, error) {
	watchCh, secretRaw, err := tx.FirstWatch(tablePeeringSecrets, indexID, id)
	if err != nil {
		return nil, fmt.Errorf("failed peering secret lookup: %w", err)
	}
	ws.Add(watchCh)

	secret, ok := secretRaw.(*pbpeering.PeeringSecrets)
	if secretRaw != nil && !ok {
		return nil, fmt.Errorf("invalid type %T", secret)
	}
	return secret, nil
}

func (s *Store) PeeringSecretsWrite(idx uint64, req *pbpeering.SecretsWriteRequest) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.peeringSecretsWriteTxn(tx, req); err != nil {
		return fmt.Errorf("failed to write peering secret: %w", err)
	}
	return tx.Commit()
}

func (s *Store) peeringSecretsWriteTxn(tx WriteTxn, req *pbpeering.SecretsWriteRequest) error {
	if req == nil || req.Request == nil {
		return nil
	}
	if err := req.Validate(); err != nil {
		return fmt.Errorf("invalid secret write request: %w", err)
	}

	peering, err := peeringReadByIDTxn(tx, nil, req.PeerID)
	if err != nil {
		return fmt.Errorf("failed to read peering by id: %w", err)
	}
	if peering == nil {
		return fmt.Errorf("unknown peering %q for secret", req.PeerID)
	}

	// If the peering came from a peering token no validation is done for the given secrets.
	// Dialing peers do not need to validate uniqueness because the secrets were generated elsewhere.
	if peering.ShouldDial() {
		r, ok := req.Request.(*pbpeering.SecretsWriteRequest_Establish)
		if !ok {
			return fmt.Errorf("invalid request type %T when persisting stream secret for dialing peer", req.Request)
		}

		secrets := pbpeering.PeeringSecrets{
			PeerID: req.PeerID,
			Stream: &pbpeering.PeeringSecrets_Stream{
				ActiveSecretID: r.Establish.ActiveStreamSecret,
			},
		}
		if err := tx.Insert(tablePeeringSecrets, &secrets); err != nil {
			return fmt.Errorf("failed inserting peering: %w", err)
		}
		return nil
	}

	// If the peering token was generated locally, validate that the newly introduced UUID is still unique.
	// RPC handlers validate that generated IDs are available, but availability cannot be guaranteed until the state store operation.
	var newSecretID string
	switch r := req.Request.(type) {

	// Establishment secrets are written when generating peering tokens, and no other secret IDs are included.
	case *pbpeering.SecretsWriteRequest_GenerateToken:
		newSecretID = r.GenerateToken.EstablishmentSecret

	// When exchanging an establishment secret a new pending stream secret is generated.
	// Active stream secrets doesn't need to be checked for uniqueness because it is only ever promoted from pending.
	case *pbpeering.SecretsWriteRequest_ExchangeSecret:
		newSecretID = r.ExchangeSecret.PendingStreamSecret
	}

	if newSecretID != "" {
		valid, err := validateProposedPeeringSecretUUIDTxn(tx, newSecretID)
		if err != nil {
			return fmt.Errorf("failed to check peering secret ID: %w", err)
		}
		if !valid {
			return fmt.Errorf("peering secret is already in use, retry the operation")
		}
		err = tx.Insert(tablePeeringSecretUUIDs, newSecretID)
		if err != nil {
			return fmt.Errorf("failed to write secret UUID: %w", err)
		}
	}

	existing, err := peeringSecretsReadByPeerIDTxn(tx, nil, req.PeerID)
	if err != nil {
		return err
	}

	secrets := pbpeering.PeeringSecrets{
		PeerID: req.PeerID,
	}

	var toDelete []string
	// Collect any overwritten UUIDs for deletion.
	switch r := req.Request.(type) {
	case *pbpeering.SecretsWriteRequest_GenerateToken:
		// Store the newly-generated establishment secret, overwriting any that existed.
		secrets.Establishment = &pbpeering.PeeringSecrets_Establishment{
			SecretID: r.GenerateToken.GetEstablishmentSecret(),
		}

		// Merge in existing stream secrets when persisting a new establishment secret.
		// This is to avoid invalidating stream secrets when a new peering token
		// is generated.
		secrets.Stream = existing.GetStream()

		// When a new token is generated we replace any un-used establishment secrets.
		if existingEstablishment := existing.GetEstablishment().GetSecretID(); existingEstablishment != "" {
			toDelete = append(toDelete, existingEstablishment)
		}

	case *pbpeering.SecretsWriteRequest_ExchangeSecret:
		if existing == nil {
			return fmt.Errorf("cannot exchange peering secret: no known secrets for peering")
		}

		// Store the newly-generated pending stream secret, overwriting any that existed.
		secrets.Stream = &pbpeering.PeeringSecrets_Stream{
			PendingSecretID: r.ExchangeSecret.GetPendingStreamSecret(),

			// Avoid invalidating existing active secrets when exchanging establishment secret for pending.
			ActiveSecretID: existing.GetStream().GetActiveSecretID(),
		}

		// When exchanging an establishment secret we invalidate the existing establishment secret.
		existingEstablishment := existing.GetEstablishment().GetSecretID()
		switch {
		case existingEstablishment == "":
			// When there is no existing establishment secret we must not proceed because another ExchangeSecret
			// RPC already invalidated it. Otherwise, this operation would overwrite the pending secret
			// from the previous ExchangeSecret.
			return fmt.Errorf("invalid establishment secret: peering was already established")

		case existingEstablishment != r.ExchangeSecret.GetEstablishmentSecret():
			// If there is an existing establishment secret but it is not the one from the request then
			// we must not proceed because a newer one was generated.
			return fmt.Errorf("invalid establishment secret")

		default:
			toDelete = append(toDelete, existingEstablishment)
		}

		// When exchanging an establishment secret unused pending secrets are overwritten.
		if existingPending := existing.GetStream().GetPendingSecretID(); existingPending != "" {
			toDelete = append(toDelete, existingPending)
		}

	case *pbpeering.SecretsWriteRequest_PromotePending:
		if existing == nil {
			return fmt.Errorf("cannot promote pending secret: no known secrets for peering")
		}
		if existing.GetStream().GetPendingSecretID() != r.PromotePending.GetActiveStreamSecret() {
			// There is a potential race if multiple dialing clusters send an Open request with a valid
			// pending secret. The secret could be validated for all concurrently at the RPC layer,
			// but then the pending secret is promoted or otherwise changes for one dialer before the others.
			return fmt.Errorf("invalid pending stream secret")
		}

		// Store the newly-generated pending stream secret, overwriting any that existed.
		secrets.Stream = &pbpeering.PeeringSecrets_Stream{
			// Promoting a pending secret moves it to active.
			PendingSecretID: "",

			// Store the newly-promoted pending secret as the active secret.
			ActiveSecretID: r.PromotePending.GetActiveStreamSecret(),
		}

		// Avoid invalidating existing establishment secrets when promoting pending secrets.
		secrets.Establishment = existing.GetEstablishment()

		// If there was previously an active stream secret it gets replaced in favor of the pending secret
		// that is being promoted.
		if existingActive := existing.GetStream().GetActiveSecretID(); existingActive != "" {
			toDelete = append(toDelete, existingActive)
		}

	case *pbpeering.SecretsWriteRequest_Establish:
		// This should never happen. Dialing peers are the only ones that can call Establish,
		// and the peering secrets for dialing peers should have been inserted earlier in the function.
		return fmt.Errorf("an accepting peer should not have called Establish RPC")

	default:
		return fmt.Errorf("got unexpected request type: %T", req.Request)
	}
	for _, id := range toDelete {
		if err := tx.Delete(tablePeeringSecretUUIDs, id); err != nil {
			return fmt.Errorf("failed to free UUID: %w", err)
		}
	}

	if err := tx.Insert(tablePeeringSecrets, &secrets); err != nil {
		return fmt.Errorf("failed inserting peering: %w", err)
	}
	return nil
}

func (s *Store) PeeringSecretsDelete(idx uint64, peerID string, dialer bool) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := peeringSecretsDeleteTxn(tx, peerID, dialer); err != nil {
		return fmt.Errorf("failed to write peering secret: %w", err)
	}
	return tx.Commit()
}

func peeringSecretsDeleteTxn(tx WriteTxn, peerID string, dialer bool) error {
	secretRaw, err := tx.First(tablePeeringSecrets, indexID, peerID)
	if err != nil {
		return fmt.Errorf("failed to fetch secret for peering: %w", err)
	}
	if secretRaw == nil {
		return nil
	}
	if err := tx.Delete(tablePeeringSecrets, secretRaw); err != nil {
		return fmt.Errorf("failed to delete secret for peering: %w", err)
	}

	// Dialing peers do not track secrets in tablePeeringSecretUUIDs.
	if dialer {
		return nil
	}

	secrets, ok := secretRaw.(*pbpeering.PeeringSecrets)
	if !ok {
		return fmt.Errorf("invalid type %T", secretRaw)
	}

	// Also clean up the UUID tracking table.
	var toDelete []string
	if establishment := secrets.GetEstablishment().GetSecretID(); establishment != "" {
		toDelete = append(toDelete, establishment)
	}
	if pending := secrets.GetStream().GetPendingSecretID(); pending != "" {
		toDelete = append(toDelete, pending)
	}
	if active := secrets.GetStream().GetActiveSecretID(); active != "" {
		toDelete = append(toDelete, active)
	}
	for _, id := range toDelete {
		if err := tx.Delete(tablePeeringSecretUUIDs, id); err != nil {
			return fmt.Errorf("failed to free UUID: %w", err)
		}
	}
	return nil
}

func (s *Store) ValidateProposedPeeringSecretUUID(id string) (bool, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return validateProposedPeeringSecretUUIDTxn(tx, id)
}

// validateProposedPeeringSecretUUIDTxn is used to test whether a candidate secretID can be used as a peering secret.
// Returns true if the given secret is not in use.
func validateProposedPeeringSecretUUIDTxn(tx ReadTxn, secretID string) (bool, error) {
	secretRaw, err := tx.First(tablePeeringSecretUUIDs, indexID, secretID)
	if err != nil {
		return false, fmt.Errorf("failed peering secret lookup: %w", err)
	}

	secret, ok := secretRaw.(string)
	if secretRaw != nil && !ok {
		return false, fmt.Errorf("invalid type %T", secret)
	}
	return secret == "", nil
}

func (s *Store) PeeringReadByID(ws memdb.WatchSet, id string) (uint64, *pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	peering, err := peeringReadByIDTxn(tx, ws, id)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read peering by id: %w", err)
	}
	if peering == nil {
		// Return the tables index so caller can watch it for changes if the peering doesn't exist
		return maxIndexWatchTxn(tx, ws, tablePeering), nil, nil
	}

	return peering.ModifyIndex, peering, nil
}

func peeringReadByIDTxn(tx ReadTxn, ws memdb.WatchSet, id string) (*pbpeering.Peering, error) {
	watchCh, peeringRaw, err := tx.FirstWatch(tablePeering, indexID, id)
	if err != nil {
		return nil, fmt.Errorf("failed peering lookup: %w", err)
	}
	ws.Add(watchCh)

	peering, ok := peeringRaw.(*pbpeering.Peering)
	if peeringRaw != nil && !ok {
		return nil, fmt.Errorf("invalid type %T", peering)
	}
	return peering, nil
}

func (s *Store) PeeringRead(ws memdb.WatchSet, q Query) (uint64, *pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return peeringReadTxn(tx, ws, q)
}

func peeringReadTxn(tx ReadTxn, ws memdb.WatchSet, q Query) (uint64, *pbpeering.Peering, error) {
	watchCh, peeringRaw, err := tx.FirstWatch(tablePeering, indexName, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering lookup: %w", err)
	}

	peering, ok := peeringRaw.(*pbpeering.Peering)
	if peeringRaw != nil && !ok {
		return 0, nil, fmt.Errorf("invalid type %T", peering)
	}
	ws.Add(watchCh)

	if peering == nil {
		// Return the tables index so caller can watch it for changes if the peering doesn't exist
		return maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeering, q.PartitionOrDefault())), nil, nil
	}

	return peering.ModifyIndex, peering, nil
}

func (s *Store) PeeringList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()
	return peeringListTxn(ws, tx, entMeta)
}

func peeringListTxn(ws memdb.WatchSet, tx ReadTxn, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error) {
	var (
		iter memdb.ResultIterator
		err  error
		idx  uint64
	)
	if entMeta.PartitionOrDefault() == structs.WildcardSpecifier {
		iter, err = tx.Get(tablePeering, indexID)
		idx = maxIndexWatchTxn(tx, ws, tablePeering)
	} else {
		iter, err = tx.Get(tablePeering, indexName+"_prefix", entMeta)
		idx = maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeering, entMeta.PartitionOrDefault()))
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering lookup: %v", err)
	}

	var result []*pbpeering.Peering
	for entry := iter.Next(); entry != nil; entry = iter.Next() {
		result = append(result, entry.(*pbpeering.Peering))
	}

	return idx, result, nil
}

func (s *Store) PeeringWrite(idx uint64, req *pbpeering.PeeringWriteRequest) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Check that the ID and Name are set.
	if req.Peering.ID == "" {
		return errors.New("Missing Peering ID")
	}
	if req.Peering.Name == "" {
		return errors.New("Missing Peering Name")
	}
	if req.Peering.State == pbpeering.PeeringState_DELETING && (req.Peering.DeletedAt == nil || structs.IsZeroProtoTime(req.Peering.DeletedAt)) {
		return errors.New("Missing deletion time for peering in deleting state")
	}
	if req.Peering.DeletedAt != nil && !structs.IsZeroProtoTime(req.Peering.DeletedAt) && req.Peering.State != pbpeering.PeeringState_DELETING {
		return fmt.Errorf("Unexpected state for peering with deletion time: %s", pbpeering.PeeringStateToAPI(req.Peering.State))
	}

	// Ensure the name is unique (cannot conflict with another peering with a different ID).
	_, existing, err := peeringReadTxn(tx, nil, Query{
		Value:          req.Peering.Name,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(req.Peering.Partition),
	})
	if err != nil {
		return err
	}

	if existing != nil {
		if req.Peering.ShouldDial() != existing.ShouldDial() {
			return fmt.Errorf("Cannot switch peering dialing mode from %t to %t", existing.ShouldDial(), req.Peering.ShouldDial())
		}

		if req.Peering.ID != existing.ID {
			return fmt.Errorf("A peering already exists with the name %q and a different ID %q", req.Peering.Name, existing.ID)
		}

		// Nothing to do if our peer wants to terminate the peering but the peering is already marked for deletion.
		if existing.State == pbpeering.PeeringState_DELETING && req.Peering.State == pbpeering.PeeringState_TERMINATED {
			return nil
		}

		// No-op deletion
		if existing.State == pbpeering.PeeringState_DELETING && req.Peering.State == pbpeering.PeeringState_DELETING {
			return nil
		}

		// No-op termination
		if existing.State == pbpeering.PeeringState_TERMINATED && req.Peering.State == pbpeering.PeeringState_TERMINATED {
			return nil
		}

		// Prevent modifications to Peering marked for deletion.
		// This blocks generating new peering tokens or re-establishing the peering until the peering is done deleting.
		if existing.State == pbpeering.PeeringState_DELETING {
			return fmt.Errorf("cannot write to peering that is marked for deletion")
		}

		if req.Peering.State == pbpeering.PeeringState_UNDEFINED {
			req.Peering.State = existing.State
		}

		// Prevent RemoteInfo from being overwritten with empty data
		if !existing.Remote.IsEmpty() && req.Peering.Remote.IsEmpty() {
			req.Peering.Remote = &pbpeering.RemoteInfo{
				Partition:  existing.Remote.Partition,
				Datacenter: existing.Remote.Datacenter,
			}
		}

		req.Peering.StreamStatus = nil
		req.Peering.CreateIndex = existing.CreateIndex
		req.Peering.ModifyIndex = idx
	} else {
		idMatch, err := peeringReadByIDTxn(tx, nil, req.Peering.ID)
		if err != nil {
			return err
		}
		if idMatch != nil {
			return fmt.Errorf("A peering already exists with the ID %q and a different name %q", req.Peering.Name, existing.ID)
		}

		if !req.Peering.IsActive() {
			return fmt.Errorf("cannot create a new peering marked for deletion")
		}
		if req.Peering.State == 0 {
			req.Peering.State = pbpeering.PeeringState_PENDING
		}
		req.Peering.CreateIndex = idx
		req.Peering.ModifyIndex = idx
	}

	// Ensure associated secrets are cleaned up when a peering is marked for deletion or terminated.
	if !req.Peering.IsActive() {
		if err := peeringSecretsDeleteTxn(tx, req.Peering.ID, req.Peering.ShouldDial()); err != nil {
			return fmt.Errorf("failed to delete peering secrets: %w", err)
		}
	}

	// Peerings are inserted before the associated StreamSecret because writing secrets
	// depends on the peering existing.
	if err := tx.Insert(tablePeering, req.Peering); err != nil {
		return fmt.Errorf("failed inserting peering: %w", err)
	}

	// Write any secrets generated with the peering.
	err = s.peeringSecretsWriteTxn(tx, req.GetSecretsRequest())
	if err != nil {
		return fmt.Errorf("failed to write peering establishment secret: %w", err)
	}

	if err := updatePeeringTableIndexes(tx, idx, req.Peering.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PeeringDelete(idx uint64, q Query) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := tx.First(tablePeering, indexName, q)
	if err != nil {
		return fmt.Errorf("failed peering lookup: %v", err)
	}

	if existing == nil {
		return nil
	}

	if existing.(*pbpeering.Peering).IsActive() {
		return fmt.Errorf("cannot delete a peering without first marking for deletion")
	}

	if err := tx.Delete(tablePeering, existing); err != nil {
		return fmt.Errorf("failed deleting peering: %v", err)
	}

	if err := updatePeeringTableIndexes(tx, idx, q.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PeeringTerminateByID(idx uint64, id string) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := peeringReadByIDTxn(tx, nil, id)
	if err != nil {
		return fmt.Errorf("failed to read peering %q: %w", id, err)
	}
	if existing == nil {
		return nil
	}

	c := proto.Clone(existing)
	clone, ok := c.(*pbpeering.Peering)
	if !ok {
		return fmt.Errorf("invalid type %T, expected *pbpeering.Peering", existing)
	}

	clone.State = pbpeering.PeeringState_TERMINATED
	clone.ModifyIndex = idx

	if err := tx.Insert(tablePeering, clone); err != nil {
		return fmt.Errorf("failed inserting peering: %w", err)
	}

	if err := updatePeeringTableIndexes(tx, idx, clone.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

// ExportedServicesForPeer returns the list of typical and proxy services
// exported to a peer.
//
// TODO(peering): What to do about terminating gateways? Sometimes terminating
// gateways are the appropriate destination to dial for an upstream mesh
// service. However, that information is handled by observing the terminating
// gateway's config entry, which we wouldn't want to replicate. How would
// client peers know to route through terminating gateways when they're not
// dialing through a remote mesh gateway?
func (s *Store) ExportedServicesForPeer(ws memdb.WatchSet, peerID string, dc string) (uint64, *structs.ExportedServiceList, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	peering, err := peeringReadByIDTxn(tx, ws, peerID)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read peering: %w", err)
	}
	if peering == nil {
		return 0, &structs.ExportedServiceList{}, nil
	}

	return exportedServicesForPeerTxn(ws, tx, peering, dc)
}

func (s *Store) ExportedServicesForAllPeersByName(ws memdb.WatchSet, dc string, entMeta acl.EnterpriseMeta) (uint64, map[string]structs.ServiceList, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	maxIdx, peerings, err := peeringListTxn(ws, tx, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to list peerings: %w", err)
	}

	out := make(map[string]structs.ServiceList)
	for _, peering := range peerings {
		idx, list, err := exportedServicesForPeerTxn(ws, tx, peering, dc)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to list exported services for peer %q: %w", peering.ID, err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		m := list.ListAllDiscoveryChains()
		if len(m) > 0 {
			sns := maps.SliceOfKeys(m)
			sort.Sort(structs.ServiceList(sns))
			out[peering.Name] = sns
		}
	}

	return maxIdx, out, nil
}

// exportedServicesForPeerTxn will find all services that are exported to a
// specific peering, and optionally include information about discovery chain
// reachable targets for these exported services if the "dc" parameter is
// specified.
func exportedServicesForPeerTxn(
	ws memdb.WatchSet,
	tx ReadTxn,
	peering *pbpeering.Peering,
	dc string,
) (uint64, *structs.ExportedServiceList, error) {
	// The DC must be specified in order to compile discovery chains.
	if dc == "" {
		return 0, nil, fmt.Errorf("datacenter cannot be empty")
	}

	maxIdx := peering.ModifyIndex

	entMeta := structs.NodeEnterpriseMetaInPartition(peering.Partition)
	idx, exportConf, err := getExportedServicesConfigEntryTxn(tx, ws, nil, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to fetch exported-services config entry: %w", err)
	}
	if idx > maxIdx {
		maxIdx = idx
	}
	if exportConf == nil {
		return maxIdx, &structs.ExportedServiceList{}, nil
	}

	var (
		// exportedServices will contain the listing of all service names that are being exported
		// and will need to be queried for connect / discovery chain information.
		exportedServices = make(map[structs.ServiceName]struct{})

		// exportedConnectServices will contain the listing of all connect service names that are being exported.
		exportedConnectServices = make(map[structs.ServiceName]struct{})

		// namespaceConnectServices provides a listing of all connect service names for a particular partition+namespace pair.
		namespaceConnectServices = make(map[acl.EnterpriseMeta]map[string]struct{})

		// namespaceDiscoChains provides a listing of all disco chain names for a particular partition+namespace pair.
		namespaceDiscoChains = make(map[acl.EnterpriseMeta]map[string]struct{})
	)

	// Helper function for inserting data and auto-creating maps.
	insertEntry := func(m map[acl.EnterpriseMeta]map[string]struct{}, entMeta acl.EnterpriseMeta, name string) {
		names, ok := m[entMeta]
		if !ok {
			names = make(map[string]struct{})
			m[entMeta] = names
		}
		names[name] = struct{}{}
	}

	// Build the set of all services that will be exported.
	// Any possible namespace wildcards or "consul" services should be removed by this step.
	for _, svc := range exportConf.Services {
		// Prevent exporting the "consul" service.
		if svc.Name == structs.ConsulServiceName {
			continue
		}
		svcEntMeta := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), svc.Namespace)
		svcName := structs.NewServiceName(svc.Name, &svcEntMeta)

		peerFound := false
		for _, consumer := range svc.Consumers {
			if consumer.Peer == peering.Name {
				peerFound = true
				break
			}
		}
		// Only look for more information if the matching peer was found.
		if !peerFound {
			continue
		}

		// If this isn't a wildcard, we can simply add it to the list of services to watch and move to the next entry.
		if svc.Name != structs.WildcardSpecifier {
			exportedServices[svcName] = struct{}{}
			continue
		}

		// If all services in the namespace are exported by the wildcard, query those service names.
		idx, typicalServices, err := serviceNamesOfKindTxn(tx, ws, structs.ServiceKindTypical, svcEntMeta)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get typical service names: %w", err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		for _, sn := range typicalServices {
			// Prevent exporting the "consul" service.
			if sn.Service.Name != structs.ConsulServiceName {
				exportedServices[sn.Service] = struct{}{}
			}
		}

		// List all config entries of kind service-resolver, service-router, service-splitter, because they
		// will be exported as connect services.
		idx, discoChains, err := listDiscoveryChainNamesTxn(tx, ws, nil, svcEntMeta)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get discovery chain names: %w", err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		for _, sn := range discoChains {
			// Prevent exporting the "consul" service.
			if sn.Name != structs.ConsulServiceName {
				exportedConnectServices[sn] = struct{}{}
				insertEntry(namespaceDiscoChains, svcEntMeta, sn.Name)
			}
		}
	}

	// At least one of the following should be true for a name to replicate it as a *connect* service:
	// - are a discovery chain by definition (service-router, service-splitter, service-resolver)
	// - have an explicit sidecar kind=connect-proxy
	// - use connect native mode
	// - are registered with a terminating gateway
	populateConnectService := func(sn structs.ServiceName) error {
		// Load all disco-chains in this namespace if we haven't already.
		if _, ok := namespaceDiscoChains[sn.EnterpriseMeta]; !ok {
			// Check to see if we have a discovery chain with the same name.
			idx, chains, err := listDiscoveryChainNamesTxn(tx, ws, nil, sn.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("failed to get connect services: %w", err)
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			for _, sn := range chains {
				insertEntry(namespaceDiscoChains, sn.EnterpriseMeta, sn.Name)
			}
		}
		// Check to see if we have the connect service.
		if _, ok := namespaceDiscoChains[sn.EnterpriseMeta][sn.Name]; ok {
			exportedConnectServices[sn] = struct{}{}
			// Do not early return because we have multiple watches that should be established.
		}

		// Load all services in this namespace if we haven't already.
		if _, ok := namespaceConnectServices[sn.EnterpriseMeta]; !ok {
			// This is more efficient than querying the service instance table.
			idx, connectServices, err := serviceNamesOfKindTxn(tx, ws, structs.ServiceKindConnectEnabled, sn.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("failed to get connect services: %w", err)
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			for _, ksn := range connectServices {
				insertEntry(namespaceConnectServices, sn.EnterpriseMeta, ksn.Service.Name)
			}
		}
		// Check to see if we have the connect service.
		if _, ok := namespaceConnectServices[sn.EnterpriseMeta][sn.Name]; ok {
			exportedConnectServices[sn] = struct{}{}
			// Do not early return because we have multiple watches that should be established.
		}

		// Check if the service is exposed via terminating gateways.
		svcGateways, err := tx.Get(tableGatewayServices, indexService, sn)
		if err != nil {
			return fmt.Errorf("failed gateway lookup for %q: %w", sn.Name, err)
		}
		ws.Add(svcGateways.WatchCh())
		for svc := svcGateways.Next(); svc != nil; svc = svcGateways.Next() {
			gs, ok := svc.(*structs.GatewayService)
			if !ok {
				return fmt.Errorf("failed converting to GatewayService for %q", sn.Name)
			}
			if gs.GatewayKind == structs.ServiceKindTerminatingGateway {
				exportedConnectServices[sn] = struct{}{}
				break
			}
		}

		return nil
	}

	// Perform queries and check if each service is connect-enabled.
	for sn := range exportedServices {
		// Do not query for data if we already know it's a connect service.
		if _, ok := exportedConnectServices[sn]; ok {
			continue
		}
		if err := populateConnectService(sn); err != nil {
			return 0, nil, err
		}
	}

	// Fetch the protocol / targets for connect services.
	chainInfo := make(map[structs.ServiceName]structs.ExportedDiscoveryChainInfo)
	populateChainInfo := func(svc structs.ServiceName) error {
		if _, ok := chainInfo[svc]; ok {
			return nil // already processed
		}

		var info structs.ExportedDiscoveryChainInfo

		idx, protocol, err := protocolForService(tx, ws, svc)
		if err != nil {
			return fmt.Errorf("failed to get protocol for service %q: %w", svc, err)
		}

		if idx > maxIdx {
			maxIdx = idx
		}
		info.Protocol = protocol

		idx, targets, err := discoveryChainOriginalTargetsTxn(tx, ws, dc, svc.Name, &svc.EnterpriseMeta)
		if err != nil {
			return fmt.Errorf("failed to get discovery chain targets for service %q: %w", svc, err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}

		// Prevent the consul service from being exported by a discovery chain.
		for _, t := range targets {
			if t.Service == structs.ConsulServiceName {
				return nil
			}
		}

		// We only need to populate the targets for replication purposes for L4 protocols, which
		// do not ultimately get intercepted by the mesh gateways.
		if !structs.IsProtocolHTTPLike(protocol) {
			sort.Slice(targets, func(i, j int) bool {
				return targets[i].ID < targets[j].ID
			})

			info.TCPTargets = targets
		}

		chainInfo[svc] = info
		return nil
	}

	for svc := range exportedConnectServices {
		if err := populateChainInfo(svc); err != nil {
			return 0, nil, err
		}
	}

	sortedServices := maps.SliceOfKeys(exportedServices)
	structs.ServiceList(sortedServices).Sort()

	list := &structs.ExportedServiceList{
		Services:    sortedServices,
		DiscoChains: chainInfo,
	}

	return maxIdx, list, nil
}

func listAllExportedServices(
	ws memdb.WatchSet,
	tx ReadTxn,
	overrides map[configentry.KindName]structs.ConfigEntry,
	entMeta acl.EnterpriseMeta,
) (uint64, map[structs.ServiceName]struct{}, error) {
	idx, export, err := getExportedServicesConfigEntryTxn(tx, ws, overrides, &entMeta)
	if err != nil {
		return 0, nil, err
	}

	found := make(map[structs.ServiceName]struct{})
	if export == nil {
		return idx, found, nil
	}

	_, services, err := listServicesExportedToAnyPeerByConfigEntry(ws, tx, export, overrides)
	if err != nil {
		return 0, nil, err
	}
	for _, svc := range services {
		found[svc] = struct{}{}
	}

	return idx, found, nil
}

//nolint:unparam
func listServicesExportedToAnyPeerByConfigEntry(
	ws memdb.WatchSet,
	tx ReadTxn,
	conf *structs.ExportedServicesConfigEntry,
	overrides map[configentry.KindName]structs.ConfigEntry,
) (uint64, []structs.ServiceName, error) {
	var (
		entMeta = conf.GetEnterpriseMeta()
		found   = make(map[structs.ServiceName]struct{})
		maxIdx  uint64
	)

	for _, svc := range conf.Services {
		svcMeta := acl.NewEnterpriseMetaWithPartition(entMeta.PartitionOrDefault(), svc.Namespace)

		sawPeer := false
		for _, consumer := range svc.Consumers {
			if consumer.Peer == "" {
				continue
			}
			sawPeer = true

			sn := structs.NewServiceName(svc.Name, &svcMeta)
			if _, ok := found[sn]; ok {
				continue
			}

			if svc.Name != structs.WildcardSpecifier {
				found[sn] = struct{}{}
			}
		}

		if sawPeer && svc.Name == structs.WildcardSpecifier {
			idx, discoChains, err := listDiscoveryChainNamesTxn(tx, ws, overrides, svcMeta)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to get discovery chain names: %w", err)
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			for _, sn := range discoChains {
				found[sn] = struct{}{}
			}
		}
	}

	foundKeys := maps.SliceOfKeys(found)

	structs.ServiceList(foundKeys).Sort()

	return maxIdx, foundKeys, nil
}

// PeeringsForService returns the list of peerings that are associated with the service name provided in the query.
// This is used to configure connect proxies for a given service. The result is generated by querying for exported
// service config entries and filtering for those that match the given service.
//
// TODO(peering): this implementation does all of the work on read to materialize this list of peerings, we should explore
// writing to a separate index that has service peerings prepared ahead of time should this become a performance bottleneck.
func (s *Store) PeeringsForService(ws memdb.WatchSet, serviceName string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return peeringsForServiceTxn(tx, ws, serviceName, entMeta)
}

func peeringsForServiceTxn(tx ReadTxn, ws memdb.WatchSet, serviceName string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.Peering, error) {
	// Return the idx of the config entry so the caller can watch for changes.
	maxIdx, peerNames, err := peersForServiceTxn(tx, ws, serviceName, &entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read peers for service name %q: %w", serviceName, err)
	}

	var peerings []*pbpeering.Peering

	// Lookup and return the peering corresponding to each name.
	for _, name := range peerNames {
		readQuery := Query{
			Value:          name,
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(entMeta.PartitionOrDefault()),
		}
		idx, peering, err := peeringReadTxn(tx, ws, readQuery)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read peering: %w", err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		if !peering.IsActive() {
			continue
		}
		peerings = append(peerings, peering)
	}
	return maxIdx, peerings, nil
}

// TrustBundleListByService returns the trust bundles for all peers that the
// given service is exported to, via a discovery chain target.
func (s *Store) TrustBundleListByService(ws memdb.WatchSet, service, dc string, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	realSvc := structs.NewServiceName(service, &entMeta)

	maxIdx, chainNames, err := s.discoveryChainSourcesTxn(tx, ws, dc, realSvc)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to list all discovery chains referring to %q: %w", realSvc, err)
	}

	peerNames := make(map[string]struct{})
	for _, chainSvc := range chainNames {
		idx, peers, err := peeringsForServiceTxn(tx, ws, chainSvc.Name, chainSvc.EnterpriseMeta)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get peers for service %s: %v", chainSvc, err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		for _, peer := range peers {
			peerNames[peer.Name] = struct{}{}
		}
	}
	peerNamesSlice := maps.SliceOfKeys(peerNames)
	sort.Strings(peerNamesSlice)

	var resp []*pbpeering.PeeringTrustBundle
	for _, peerName := range peerNamesSlice {
		pq := Query{
			Value:          strings.ToLower(peerName),
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(entMeta.PartitionOrDefault()),
		}
		idx, trustBundle, err := peeringTrustBundleReadTxn(tx, ws, pq)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read trust bundle for peer %s: %v", peerName, err)
		}
		if idx > maxIdx {
			maxIdx = idx
		}
		if trustBundle != nil {
			resp = append(resp, trustBundle)
		}
	}

	return maxIdx, resp, nil
}

// PeeringTrustBundleList returns the peering trust bundles for all peers.
func (s *Store) PeeringTrustBundleList(ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return peeringTrustBundleListTxn(tx, ws, entMeta)
}

func peeringTrustBundleListTxn(tx ReadTxn, ws memdb.WatchSet, entMeta acl.EnterpriseMeta) (uint64, []*pbpeering.PeeringTrustBundle, error) {
	iter, err := tx.Get(tablePeeringTrustBundles, indexID+"_prefix", entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering trust bundle lookup: %w", err)
	}

	idx := maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeeringTrustBundles, entMeta.PartitionOrDefault()))

	var result []*pbpeering.PeeringTrustBundle
	for entry := iter.Next(); entry != nil; entry = iter.Next() {
		result = append(result, entry.(*pbpeering.PeeringTrustBundle))
	}

	return idx, result, nil
}

// PeeringTrustBundleRead returns the peering trust bundle for the peer name given as the query value.
func (s *Store) PeeringTrustBundleRead(ws memdb.WatchSet, q Query) (uint64, *pbpeering.PeeringTrustBundle, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return peeringTrustBundleReadTxn(tx, ws, q)
}

func peeringTrustBundleReadTxn(tx ReadTxn, ws memdb.WatchSet, q Query) (uint64, *pbpeering.PeeringTrustBundle, error) {
	watchCh, ptbRaw, err := tx.FirstWatch(tablePeeringTrustBundles, indexID, q)
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering trust bundle lookup: %w", err)
	}

	ptb, ok := ptbRaw.(*pbpeering.PeeringTrustBundle)
	if ptbRaw != nil && !ok {
		return 0, nil, fmt.Errorf("invalid type %T", ptb)
	}
	ws.Add(watchCh)

	if ptb == nil {
		// Return the tables index so caller can watch it for changes if the trust bundle doesn't exist
		return maxIndexWatchTxn(tx, ws, partitionedIndexEntryName(tablePeeringTrustBundles, q.PartitionOrDefault())), nil, nil
	}
	return ptb.ModifyIndex, ptb, nil
}

// PeeringTrustBundleWrite writes ptb to the state store.
// It also updates the corresponding peering object with the new certs.
// If there is an existing trust bundle with the given peer name, it will be overwritten.
// If there is no corresponding peering, then an error is returned.
func (s *Store) PeeringTrustBundleWrite(idx uint64, ptb *pbpeering.PeeringTrustBundle) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if ptb.PeerName == "" {
		return errors.New("missing peer name")
	}

	// Check for the existence of the peering object
	_, existingPeering, err := peeringReadTxn(tx, nil, Query{
		Value:          ptb.PeerName,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(ptb.Partition),
	})
	if err != nil {
		return err
	}
	if existingPeering == nil {
		return fmt.Errorf("cannot write peering trust bundle for unknown peering %s", ptb.PeerName)
	}
	// Prevent modifications to Peering marked for deletion.
	// This blocks generating new peering tokens or re-establishing the peering until the peering is done deleting.
	if existingPeering.State == pbpeering.PeeringState_DELETING {
		return fmt.Errorf("cannot write to peering that is marked for deletion")
	}
	c := proto.Clone(existingPeering)
	clone, ok := c.(*pbpeering.Peering)
	if !ok {
		return fmt.Errorf("invalid type %T, expected *pbpeering.Peering", clone)
	}

	// Update the certs on the peering
	rootPEMs := make([]string, 0, len(ptb.RootPEMs))
	for _, c := range ptb.RootPEMs {
		rootPEMs = append(rootPEMs, lib.EnsureTrailingNewline(c))
	}
	clone.PeerCAPems = rootPEMs
	clone.ModifyIndex = idx

	if err := tx.Insert(tablePeering, clone); err != nil {
		return fmt.Errorf("failed inserting peering: %w", err)
	}
	if err := updatePeeringTableIndexes(tx, idx, clone.PartitionOrDefault()); err != nil {
		return err
	}

	// Check for the existing trust bundle and update
	q := Query{
		Value:          ptb.PeerName,
		EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(ptb.Partition),
	}
	existingRaw, err := tx.First(tablePeeringTrustBundles, indexID, q)
	if err != nil {
		return fmt.Errorf("failed peering trust bundle lookup: %w", err)
	}

	existingPTB, ok := existingRaw.(*pbpeering.PeeringTrustBundle)
	if existingRaw != nil && !ok {
		return fmt.Errorf("invalid type %T", existingRaw)
	}

	if existingPTB != nil {
		ptb.CreateIndex = existingPTB.CreateIndex

	} else {
		ptb.CreateIndex = idx
	}

	ptb.ModifyIndex = idx

	if err := tx.Insert(tablePeeringTrustBundles, ptb); err != nil {
		return fmt.Errorf("failed inserting peering trust bundle: %w", err)
	}

	if err := updatePeeringTrustBundlesTableIndexes(tx, idx, ptb.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PeeringTrustBundleDelete(idx uint64, q Query) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	existing, err := tx.First(tablePeeringTrustBundles, indexID, q)
	if err != nil {
		return fmt.Errorf("failed peering trust bundle lookup: %v", err)
	}

	if existing == nil {
		return nil
	}

	if err := tx.Delete(tablePeeringTrustBundles, existing); err != nil {
		return fmt.Errorf("failed deleting peering trust bundle: %v", err)
	}

	if err := updatePeeringTrustBundlesTableIndexes(tx, idx, q.PartitionOrDefault()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Snapshot) Peerings() (memdb.ResultIterator, error) {
	return s.tx.Get(tablePeering, indexName)
}

func (s *Snapshot) PeeringTrustBundles() (memdb.ResultIterator, error) {
	return s.tx.Get(tablePeeringTrustBundles, indexID)
}

func (s *Snapshot) PeeringSecrets() (memdb.ResultIterator, error) {
	return s.tx.Get(tablePeeringSecrets, indexID)
}

func (r *Restore) Peering(p *pbpeering.Peering) error {
	if err := r.tx.Insert(tablePeering, p); err != nil {
		return fmt.Errorf("failed restoring peering: %w", err)
	}

	if err := updatePeeringTableIndexes(r.tx, p.ModifyIndex, p.PartitionOrDefault()); err != nil {
		return err
	}

	return nil
}

func (r *Restore) PeeringTrustBundle(ptb *pbpeering.PeeringTrustBundle) error {
	if err := r.tx.Insert(tablePeeringTrustBundles, ptb); err != nil {
		return fmt.Errorf("failed restoring peering trust bundle: %w", err)
	}
	if err := updatePeeringTrustBundlesTableIndexes(r.tx, ptb.ModifyIndex, ptb.PartitionOrDefault()); err != nil {
		return err
	}
	return nil
}

func (r *Restore) PeeringSecrets(p *pbpeering.PeeringSecrets) error {
	if err := r.tx.Insert(tablePeeringSecrets, p); err != nil {
		return fmt.Errorf("failed restoring peering secrets: %w", err)
	}

	var uuids []string
	if establishment := p.GetEstablishment().GetSecretID(); establishment != "" {
		uuids = append(uuids, establishment)
	}
	if pending := p.GetStream().GetPendingSecretID(); pending != "" {
		uuids = append(uuids, pending)
	}
	if active := p.GetStream().GetActiveSecretID(); active != "" {
		uuids = append(uuids, active)
	}

	for _, id := range uuids {
		if err := r.tx.Insert(tablePeeringSecretUUIDs, id); err != nil {
			return fmt.Errorf("failed restoring peering secret UUIDs: %w", err)
		}
	}
	return nil
}

// peersForServiceTxn returns the names of all peers that a service is exported to.
func peersForServiceTxn(
	tx ReadTxn,
	ws memdb.WatchSet,
	serviceName string,
	entMeta *acl.EnterpriseMeta,
) (uint64, []string, error) {
	// Exported service config entries are scoped to partitions so they are in the default namespace.
	partitionMeta := structs.DefaultEnterpriseMetaInPartition(entMeta.PartitionOrDefault())

	idx, rawEntry, err := configEntryTxn(tx, ws, structs.ExportedServices, partitionMeta.PartitionOrDefault(), partitionMeta)
	if err != nil {
		return 0, nil, err
	}
	if rawEntry == nil {
		return idx, nil, err
	}

	entry, ok := rawEntry.(*structs.ExportedServicesConfigEntry)
	if !ok {
		return 0, nil, fmt.Errorf("unexpected type %T for pbpeering.Peering index", rawEntry)
	}

	var (
		wildcardNamespaceIdx = -1
		wildcardServiceIdx   = -1
		exactMatchIdx        = -1
	)

	// Ensure the metadata is defaulted since we make assertions against potentially empty values below.
	// In CE this is a no-op.
	if entMeta == nil {
		entMeta = acl.DefaultEnterpriseMeta()
	}
	entMeta.Normalize()

	// Services can be exported via wildcards or by their exact name:
	// 		Namespace: *,     Service: *
	// 		Namespace: Exact, Service: *
	// 		Namespace: Exact, Service: Exact
	for i, service := range entry.Services {
		switch {
		case service.Namespace == structs.WildcardSpecifier:
			wildcardNamespaceIdx = i

		case service.Name == structs.WildcardSpecifier && acl.EqualNamespaces(service.Namespace, entMeta.NamespaceOrDefault()):
			wildcardServiceIdx = i

		case service.Name == serviceName && acl.EqualNamespaces(service.Namespace, entMeta.NamespaceOrDefault()):
			exactMatchIdx = i
		}
	}

	var results []string

	// Prefer the exact match over the wildcard match. This matches how we handle intention precedence.
	var targetIdx int
	switch {
	case exactMatchIdx >= 0:
		targetIdx = exactMatchIdx

	case wildcardServiceIdx >= 0:
		targetIdx = wildcardServiceIdx

	case wildcardNamespaceIdx >= 0:
		targetIdx = wildcardNamespaceIdx

	default:
		return idx, results, nil
	}

	for _, c := range entry.Services[targetIdx].Consumers {
		if c.Peer != "" {
			results = append(results, c.Peer)
		}
	}
	return idx, results, nil
}

func (s *Store) PeeringListDeleted(ws memdb.WatchSet) (uint64, []*pbpeering.Peering, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	return peeringListDeletedTxn(tx, ws)
}

func peeringListDeletedTxn(tx ReadTxn, ws memdb.WatchSet) (uint64, []*pbpeering.Peering, error) {
	iter, err := tx.Get(tablePeering, indexDeleted, BoolQuery{Value: true})
	if err != nil {
		return 0, nil, fmt.Errorf("failed peering lookup: %v", err)
	}

	// Instead of watching iter.WatchCh() we only need to watch the index entry for the peering table
	// This is sufficient to pick up any changes to peerings.
	idx := maxIndexWatchTxn(tx, ws, tablePeering)

	var result []*pbpeering.Peering
	for t := iter.Next(); t != nil; t = iter.Next() {
		result = append(result, t.(*pbpeering.Peering))
	}

	return idx, result, nil
}
