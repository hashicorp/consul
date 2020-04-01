// +build !consulent

package state

import (
	"fmt"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// servicesTableSchema returns a new table schema used to store information
// about services.
func servicesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "services",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "ServiceID",
							Lowercase: true,
						},
					},
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ServiceName",
					Lowercase: true,
				},
			},
			"connect": &memdb.IndexSchema{
				Name:         "connect",
				AllowMissing: true,
				Unique:       false,
				Indexer:      &IndexConnectService{},
			},
			"kind": &memdb.IndexSchema{
				Name:         "kind",
				AllowMissing: false,
				Unique:       false,
				Indexer:      &IndexServiceKind{},
			},
		},
	}
}

// checksTableSchema returns a new table schema used for storing and indexing
// health check information. Health checks have a number of different attributes
// we want to filter by, so this table is a bit more complex.
func checksTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "checks",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "CheckID",
							Lowercase: true,
						},
					},
				},
			},
			"status": &memdb.IndexSchema{
				Name:         "status",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Status",
					Lowercase: false,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "ServiceName",
					Lowercase: true,
				},
			},
			"node": &memdb.IndexSchema{
				Name:         "node",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Node",
					Lowercase: true,
				},
			},
			"node_service_check": &memdb.IndexSchema{
				Name:         "node_service_check",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.FieldSetIndex{
							Field: "ServiceID",
						},
					},
				},
			},
			"node_service": &memdb.IndexSchema{
				Name:         "node_service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Node",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "ServiceID",
							Lowercase: true,
						},
					},
				},
			},
		},
	}
}

// TODO (gateways) (freddy) enterprise implementation
// terminatingGatewayServicesTableSchema returns a new table schema used to store information
// about services associated with terminating gateways.
func terminatingGatewayServicesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: "terminating-gateway-services",
		Indexes: map[string]*memdb.IndexSchema{
			"id": &memdb.IndexSchema{
				Name:         "id",
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field:     "Gateway",
							Lowercase: true,
						},
						&memdb.StringFieldIndex{
							Field:     "Service",
							Lowercase: true,
						},
					},
				},
			},
			"gateway": &memdb.IndexSchema{
				Name:         "gateway",
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Gateway",
					Lowercase: true,
				},
			},
			"service": &memdb.IndexSchema{
				Name:         "service",
				AllowMissing: true,
				Unique:       false,
				Indexer: &memdb.StringFieldIndex{
					Field:     "Service",
					Lowercase: true,
				},
			},
		},
	}
}

func serviceIndexName(name string, _ *structs.EnterpriseMeta) string {
	return fmt.Sprintf("service.%s", name)
}

func serviceKindIndexName(kind structs.ServiceKind, _ *structs.EnterpriseMeta) string {
	switch kind {
	case structs.ServiceKindTypical:
		// needs a special case here
		return "service_kind.typical"
	default:
		return "service_kind." + string(kind)
	}
}

func (s *Store) catalogUpdateServicesIndexes(tx *memdb.Txn, idx uint64, _ *structs.EnterpriseMeta) error {
	// overall services index
	if err := indexUpdateMaxTxn(tx, idx, "services"); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func (s *Store) catalogUpdateServiceKindIndexes(tx *memdb.Txn, kind structs.ServiceKind, idx uint64, _ *structs.EnterpriseMeta) error {
	// service-kind index
	if err := indexUpdateMaxTxn(tx, idx, serviceKindIndexName(kind, nil)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func (s *Store) catalogUpdateServiceIndexes(tx *memdb.Txn, serviceName string, idx uint64, _ *structs.EnterpriseMeta) error {
	// per-service index
	if err := indexUpdateMaxTxn(tx, idx, serviceIndexName(serviceName, nil)); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}

func (s *Store) catalogUpdateServiceExtinctionIndex(tx *memdb.Txn, idx uint64, _ *structs.EnterpriseMeta) error {
	if err := tx.Insert("index", &IndexEntry{serviceLastExtinctionIndexName, idx}); err != nil {
		return fmt.Errorf("failed updating missing service extinction index: %s", err)
	}
	return nil
}

func (s *Store) catalogInsertService(tx *memdb.Txn, svc *structs.ServiceNode) error {
	// Insert the service and update the index
	if err := tx.Insert("services", svc); err != nil {
		return fmt.Errorf("failed inserting service: %s", err)
	}

	if err := s.catalogUpdateServicesIndexes(tx, svc.ModifyIndex, &svc.EnterpriseMeta); err != nil {
		return err
	}

	if err := s.catalogUpdateServiceIndexes(tx, svc.ServiceName, svc.ModifyIndex, &svc.EnterpriseMeta); err != nil {
		return err
	}

	if err := s.catalogUpdateServiceKindIndexes(tx, svc.ServiceKind, svc.ModifyIndex, &svc.EnterpriseMeta); err != nil {
		return err
	}

	return nil
}

func (s *Store) catalogServicesMaxIndex(tx *memdb.Txn, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "services")
}

func (s *Store) catalogServiceMaxIndex(tx *memdb.Txn, serviceName string, _ *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch("index", "id", serviceIndexName(serviceName, nil))
}

func (s *Store) catalogServiceKindMaxIndex(tx *memdb.Txn, ws memdb.WatchSet, kind structs.ServiceKind, entMeta *structs.EnterpriseMeta) uint64 {
	return maxIndexWatchTxn(tx, ws, serviceKindIndexName(kind, nil))
}

// TODO (gateways) (freddy) enterprise implementation
func (s *Store) terminatingGatewayServicesMaxIndex(tx *memdb.Txn, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "terminating-gateway-services")
}

func (s *Store) catalogServiceList(tx *memdb.Txn, _ *structs.EnterpriseMeta, _ bool) (memdb.ResultIterator, error) {
	return tx.Get("services", "id")
}

func (s *Store) catalogServiceListByKind(tx *memdb.Txn, kind structs.ServiceKind, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("services", "kind", string(kind))
}

func (s *Store) catalogServiceListByNode(tx *memdb.Txn, node string, _ *structs.EnterpriseMeta, _ bool) (memdb.ResultIterator, error) {
	return tx.Get("services", "node", node)
}

func (s *Store) catalogServiceNodeList(tx *memdb.Txn, name string, index string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("services", index, name)
}

// TODO (gateways) (freddy) enterprise implementation
func (s *Store) serviceTerminatingGateway(tx *memdb.Txn, name string, _ *structs.EnterpriseMeta) (interface{}, error) {
	return tx.First("terminating-gateway-services", "service", name)
}

// TODO (gateways) (freddy) enterprise implementation
func (s *Store) terminatingGatewayServices(tx *memdb.Txn, name string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("terminating-gateway-services", "gateway", name)
}

func (s *Store) catalogServiceLastExtinctionIndex(tx *memdb.Txn, _ *structs.EnterpriseMeta) (interface{}, error) {
	return tx.First("index", "id", serviceLastExtinctionIndexName)
}

func (s *Store) catalogMaxIndex(tx *memdb.Txn, _ *structs.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexTxn(tx, "nodes", "services", "checks")
	}
	return maxIndexTxn(tx, "nodes", "services")
}

func (s *Store) catalogMaxIndexWatch(tx *memdb.Txn, ws memdb.WatchSet, _ *structs.EnterpriseMeta, checks bool) uint64 {
	if checks {
		return maxIndexWatchTxn(tx, ws, "nodes", "services", "checks")
	}
	return maxIndexWatchTxn(tx, ws, "nodes", "services")
}

func (s *Store) catalogUpdateCheckIndexes(tx *memdb.Txn, idx uint64, _ *structs.EnterpriseMeta) error {
	// update the universal index entry
	if err := tx.Insert("index", &IndexEntry{"checks", idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}
	return nil
}

func (s *Store) catalogChecksMaxIndex(tx *memdb.Txn, _ *structs.EnterpriseMeta) uint64 {
	return maxIndexTxn(tx, "checks")
}

func (s *Store) catalogListChecksByNode(tx *memdb.Txn, node string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("checks", "node", node)
}

func (s *Store) catalogListChecksByService(tx *memdb.Txn, service string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("checks", "service", service)
}

func (s *Store) catalogListChecksInState(tx *memdb.Txn, state string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	// simpler than normal due to the use of the CompoundMultiIndex
	return tx.Get("checks", "status", state)
}

func (s *Store) catalogListChecks(tx *memdb.Txn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("checks", "id")
}

func (s *Store) catalogListNodeChecks(tx *memdb.Txn, node string) (memdb.ResultIterator, error) {
	return tx.Get("checks", "node_service_check", node, false)
}

func (s *Store) catalogListServiceChecks(tx *memdb.Txn, node string, service string, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("checks", "node_service", node, service)
}

func (s *Store) catalogInsertCheck(tx *memdb.Txn, chk *structs.HealthCheck, idx uint64) error {
	// Insert the check
	if err := tx.Insert("checks", chk); err != nil {
		return fmt.Errorf("failed inserting check: %s", err)
	}

	if err := s.catalogUpdateCheckIndexes(tx, idx, &chk.EnterpriseMeta); err != nil {
		return err
	}

	return nil
}

func (s *Store) catalogChecksForNodeService(tx *memdb.Txn, node string, service string, entMeta *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	return tx.Get("checks", "node_service", node, service)
}

func (s *Store) validateRegisterRequestTxn(tx *memdb.Txn, args *structs.RegisterRequest) (*structs.EnterpriseMeta, error) {
	return nil, nil
}

func (s *Store) ValidateRegisterRequest(args *structs.RegisterRequest) (*structs.EnterpriseMeta, error) {
	return nil, nil
}

// TODO (gateways) (freddy) enterprise implementation
// updateGatewayService associates services with gateways as specified in a terminating-gateway config entry
func (s *Store) updateTerminatingGatewayServices(tx *memdb.Txn, idx uint64, conf structs.ConfigEntry, entMeta *structs.EnterpriseMeta) error {
	entry, ok := conf.(*structs.TerminatingGatewayConfigEntry)
	if !ok {
		return fmt.Errorf("unexpected config entry type: %T", conf)
	}

	// Delete all associated with gateway first, to avoid keeping mappings that were removed
	if _, err := tx.DeleteAll("terminating-gateway-services", "gateway", entry.Name); err != nil {
		return fmt.Errorf("failed to truncate gateway services table: %v", err)
	}

	for _, svc := range entry.Services {
		// If the service is a wildcard we need to target all services within the namespace
		if svc.Name == structs.WildcardSpecifier {
			services, err := s.catalogServiceList(tx, entMeta, false)
			if err != nil {
				return fmt.Errorf("failed querying services: %s", err)
			}

			// Iterate over them and insert
			for service := services.Next(); service != nil; service = services.Next() {
				sn := service.(*structs.ServiceNode)

				// Only associate typical non-consul services with gateways
				if sn.ServiceKind != structs.ServiceKindTypical || sn.ServiceName == "consul" {
					continue
				}

				existing, err := tx.First("terminating-gateway-services", "service", sn.ServiceName)
				if err != nil {
					return fmt.Errorf("gateway service lookup failed: %s", err)
				}

				// If there's an existing service, then we skip it.
				// This means the service was specified on its own, and the service entry overrides the wildcard entry.
				if existing != nil {
					continue
				}

				mapping := &structs.GatewayService{
					Gateway:  entry.Name,
					Service:  sn.ServiceName,
					KeyFile:  svc.KeyFile,
					CertFile: svc.CertFile,
					CAFile:   svc.CAFile,
				}
				if err := tx.Insert("terminating-gateway-services", mapping); err != nil {
					return fmt.Errorf("failed inserting gateway service mapping: %s", err)
				}
			}

			// Also store a mapping for the wildcard so that the TLS creds can be pulled
			// for new services registered in its namespace
			mapping := &structs.GatewayService{
				Gateway:  entry.Name,
				Service:  svc.Name,
				KeyFile:  svc.KeyFile,
				CertFile: svc.CertFile,
				CAFile:   svc.CAFile,
			}
			if err := tx.Insert("terminating-gateway-services", mapping); err != nil {
				return fmt.Errorf("failed inserting gateway service mapping: %s", err)
			}

			continue
		}

		// Check if the service is already associated with a gateway
		existing, err := tx.First("terminating-gateway-services", "service", svc.Name)
		if err != nil {
			return fmt.Errorf("gateway service lookup failed: %s", err)
		}
		if existing != nil {
			cfg := existing.(*structs.GatewayService)

			// Only return an error if the stored gateway does not match the one from the config entry
			if cfg.Gateway != entry.Name {
				return fmt.Errorf("service %q is associated with different gateway, %q", svc.Name, cfg.Gateway)
			}
		}

		// Since this service was specified on its own, and not with a wildcard,
		// if there is an existing entry, we overwrite it. The service entry is the source of truth.
		//
		// By extension, if TLS creds are provided with a wildcard but are not provided in
		// the service entry, the service does not inherit the creds from the wildcard.
		mapping := &structs.GatewayService{
			Gateway:  entry.Name,
			Service:  svc.Name,
			KeyFile:  svc.KeyFile,
			CertFile: svc.CertFile,
			CAFile:   svc.CAFile,
		}
		if err := tx.Insert("terminating-gateway-services", mapping); err != nil {
			return fmt.Errorf("failed inserting gateway service mapping: %s", err)
		}
	}

	if err := indexUpdateMaxTxn(tx, idx, "terminating-gateway-services"); err != nil {
		return fmt.Errorf("failed updating terminating-gateway-services index: %v", err)
	}
	return nil
}

// TODO (gateways) (freddy) enterprise implementation
// updateGatewayService associates services with gateways after an eligible event
// ie. Registering a service in a namespace targeted by a gateway
func (s *Store) updateTerminatingGatewayService(tx *memdb.Txn, idx uint64, gateway string, service string, entMeta *structs.EnterpriseMeta) error {
	mapping := &structs.GatewayService{
		Gateway: gateway,
		Service: service,
	}

	// If a wildcard specifier is registered for that namespace, use its TLS config
	wc, err := tx.First("terminating-gateway-services", "service", structs.WildcardSpecifier)
	if err != nil {
		return fmt.Errorf("gateway service lookup failed: %s", err)
	}
	if wc != nil {
		cfg := wc.(*structs.GatewayService)
		mapping.CAFile = cfg.CAFile
		mapping.CertFile = cfg.CertFile
		mapping.KeyFile = cfg.KeyFile
	}

	if err := tx.Insert("terminating-gateway-services", mapping); err != nil {
		return fmt.Errorf("failed inserting gateway service mapping: %s", err)
	}

	if err := indexUpdateMaxTxn(tx, idx, "terminating-gateway-services"); err != nil {
		return fmt.Errorf("failed updating terminating-gateway-services index: %v", err)
	}
	return nil
}
