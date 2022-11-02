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
	"github.com/hashicorp/consul/sdk/testutil"
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
		ACLResolver:    newStaticResolver(acl.ManageAll()),
		ViewStore:      store,
		EventPublisher: publisher,
		Logger:         logger,
	})

	eventCh := make(chan proxycfg.UpdateEvent)
	require.NoError(t, intentions.Notify(ctx, &structs.ServiceSpecificRequest{
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
		ServiceName:    serviceName,
	}, "", eventCh))

	testutil.RunStep(t, "initial snapshot", func(t *testing.T) {
		getEventResult[structs.Intentions](t, eventCh)
	})

	testutil.RunStep(t, "publish a namespace-wildcard partition", func(t *testing.T) {
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

		result := getEventResult[structs.Intentions](t, eventCh)
		require.Len(t, result, 1)
	})
}
