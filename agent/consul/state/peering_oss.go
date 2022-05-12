//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/proto/pbpeering"
)

func indexPeeringFromQuery(raw interface{}) ([]byte, error) {
	q, ok := raw.(Query)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for Query index", raw)
	}

	var b indexBuilder
	b.String(strings.ToLower(q.Value))
	return b.Bytes(), nil
}

func indexFromPeering(raw interface{}) ([]byte, error) {
	p, ok := raw.(*pbpeering.Peering)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for structs.Peering index", raw)
	}

	if p.Name == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(p.Name))
	return b.Bytes(), nil
}

func indexFromPeeringTrustBundle(raw interface{}) ([]byte, error) {
	ptb, ok := raw.(*pbpeering.PeeringTrustBundle)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T for pbpeering.PeeringTrustBundle index", raw)
	}

	if ptb.PeerName == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(ptb.PeerName))
	return b.Bytes(), nil
}

func updatePeeringTableIndexes(tx WriteTxn, idx uint64, _ string) error {
	if err := tx.Insert(tableIndex, &IndexEntry{Key: tablePeering, Value: idx}); err != nil {
		return fmt.Errorf("failed updating table index: %w", err)
	}
	return nil
}

func updatePeeringTrustBundlesTableIndexes(tx WriteTxn, idx uint64, _ string) error {
	if err := tx.Insert(tableIndex, &IndexEntry{Key: tablePeeringTrustBundles, Value: idx}); err != nil {
		return fmt.Errorf("failed updating table index: %w", err)
	}
	return nil
}
