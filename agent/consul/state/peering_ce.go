//go:build !consulent
// +build !consulent

package state

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/proto/pbpeering"
)

func indexPeeringFromQuery(q Query) ([]byte, error) {
	var b indexBuilder
	b.String(strings.ToLower(q.Value))
	return b.Bytes(), nil
}

func indexFromPeering(p *pbpeering.Peering) ([]byte, error) {
	if p.Name == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(p.Name))
	return b.Bytes(), nil
}

func indexFromPeeringTrustBundle(ptb *pbpeering.PeeringTrustBundle) ([]byte, error) {
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
