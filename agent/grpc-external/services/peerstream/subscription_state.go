package peerstream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbservice"
)

// subscriptionState is a collection of working state tied to a peerID subscription.
type subscriptionState struct {
	// peerName is immutable and is the LOCAL name for the peering
	peerName string
	// partition is immutable
	partition string

	// plain data
	exportList *structs.ExportedServiceList

	watchedServices map[structs.ServiceName]context.CancelFunc
	connectServices map[structs.ServiceName]structs.ExportedDiscoveryChainInfo

	// eventVersions is a duplicate event suppression system keyed by the "id"
	// not the "correlationID"
	eventVersions map[string]string

	meshGateway *pbservice.IndexedCheckServiceNodes

	// updateCh is an internal implementation detail for the machinery of the
	// manager.
	updateCh chan<- cache.UpdateEvent

	// publicUpdateCh is the channel the manager uses to pass data back to the
	// caller.
	publicUpdateCh chan<- cache.UpdateEvent
}

func newSubscriptionState(peerName, partition string) *subscriptionState {
	return &subscriptionState{
		peerName:        peerName,
		partition:       partition,
		watchedServices: make(map[structs.ServiceName]context.CancelFunc),
		connectServices: make(map[structs.ServiceName]structs.ExportedDiscoveryChainInfo),
		eventVersions:   make(map[string]string),
	}
}

func (s *subscriptionState) sendPendingEvents(
	ctx context.Context,
	logger hclog.Logger,
	pending *pendingPayload,
) {
	for _, pendingEvt := range pending.Events {
		cID := pendingEvt.CorrelationID
		newVersion := pendingEvt.Version

		oldVersion, ok := s.eventVersions[pendingEvt.ID]
		if ok && newVersion == oldVersion {
			logger.Trace("skipping send of duplicate public event", "correlationID", cID)
			continue
		}

		logger.Trace("sending public event", "correlationID", cID)
		s.eventVersions[pendingEvt.ID] = newVersion

		evt := cache.UpdateEvent{
			CorrelationID: cID,
			Result:        pendingEvt.Result,
		}

		select {
		case s.publicUpdateCh <- evt:
		case <-ctx.Done():
		}
	}
}

func (s *subscriptionState) cleanupEventVersions(logger hclog.Logger) {
	for id := range s.eventVersions {
		keep := false
		switch {
		case id == meshGatewayPayloadID:
			keep = true

		case id == caRootsPayloadID:
			keep = true

		case id == serverAddrsPayloadID:
			keep = true

		case id == exportedServiceListID:
			keep = true

		case strings.HasPrefix(id, servicePayloadIDPrefix):
			name := strings.TrimPrefix(id, servicePayloadIDPrefix)
			sn := structs.ServiceNameFromString(name)

			if _, ok := s.watchedServices[sn]; ok {
				keep = true
			}

		case strings.HasPrefix(id, discoveryChainPayloadIDPrefix):
			name := strings.TrimPrefix(id, discoveryChainPayloadIDPrefix)
			sn := structs.ServiceNameFromString(name)

			if _, ok := s.connectServices[sn]; ok {
				keep = true
			}
		}

		if !keep {
			logger.Trace("cleaning up unreferenced event id version", "id", id)
			delete(s.eventVersions, id)
		}
	}
}

type pendingPayload struct {
	Events []pendingEvent
}

type pendingEvent struct {
	ID            string
	CorrelationID string
	Result        proto.Message
	Version       string
}

const (
	serverAddrsPayloadID          = "server-addrs"
	caRootsPayloadID              = "roots"
	meshGatewayPayloadID          = "mesh-gateway"
	exportedServiceListID         = "exported-service-list"
	servicePayloadIDPrefix        = "service:"
	discoveryChainPayloadIDPrefix = "chain:"
)

func (p *pendingPayload) Add(id string, correlationID string, raw interface{}) error {
	result, ok := raw.(proto.Message)
	if !ok {
		return fmt.Errorf("invalid type for %q event: %T", correlationID, raw)
	}

	version, err := hashProtobuf(result)
	if err != nil {
		return fmt.Errorf("error hashing %q event: %w", correlationID, err)
	}

	p.Events = append(p.Events, pendingEvent{
		ID:            id,
		CorrelationID: correlationID,
		Result:        result,
		Version:       version,
	})

	return nil
}

func hashProtobuf(res proto.Message) (string, error) {
	h := sha256.New()
	buffer := proto.NewBuffer(nil)
	buffer.SetDeterministic(true)

	err := buffer.Marshal(res)
	if err != nil {
		return "", err
	}
	h.Write(buffer.Bytes())
	buffer.Reset()

	return hex.EncodeToString(h.Sum(nil)), nil
}
