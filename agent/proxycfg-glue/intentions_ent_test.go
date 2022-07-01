package proxycfgglue

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

func TestServerIntentions_Enterprise(t *testing.T) {
	// This test asserts that we also subscribe to the wildcard namespace intention.
	const (
		serviceName = "web"
		index       = 1
	)

	logger := hclog.NewNullLogger()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := submatview.NewStore(logger)
	go store.Run(ctx)

	publisher := stream.NewEventPublisher(10 * time.Second)
	publisher.RegisterHandler(pbsubscribe.Topic_ServiceIntentions,
		func(stream.SubscribeRequest, stream.SnapshotAppender) (uint64, error) { return index, nil },
		false)
	go publisher.Run(ctx)

	intentions := ServerIntentions(ServerDataSourceDeps{
		ACLResolver:    staticResolver{acl.ManageAll()},
		ViewStore:      store,
		EventPublisher: publisher,
		Logger:         logger,
	})

	eventCh := make(chan proxycfg.UpdateEvent)
	require.NoError(t, intentions.Notify(ctx, &structs.ServiceSpecificRequest{
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		ServiceName:    serviceName,
	}, "", eventCh))

	// Wait for the initial snapshots.
	select {
	case <-eventCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	// Publish a namespace wildcard intention.
	publisher.Publish([]stream.Event{
		{
			Topic: pbsubscribe.Topic_ServiceIntentions,
			Index: index + 1,
			Payload: state.EventPayloadConfigEntry{
				Op: pbsubscribe.ConfigEntryUpdate_Upsert,
				Value: &structs.ServiceIntentionsConfigEntry{
					Name:           structs.WildcardSpecifier,
					EnterpriseMeta: *acl.WildcardEnterpriseMeta(),
					Sources: []*structs.SourceIntention{
						{Name: structs.WildcardSpecifier, Action: structs.IntentionActionAllow, Precedence: 1},
					},
				},
			},
		},
	})

	select {
	case event := <-eventCh:
		result, ok := event.Result.(structs.Intentions)
		require.Truef(t, ok, "expected Intentions, got: %T", event.Result)
		require.Len(t, result, 1)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}
