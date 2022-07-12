package proxycfgglue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerIntentions(t *testing.T) {
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
		ServiceName:    serviceName,
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}, "", eventCh))

	testutil.RunStep(t, "initial snapshot", func(t *testing.T) {
		getEventResult[structs.Intentions](t, eventCh)
	})

	testutil.RunStep(t, "publishing an explicit intention", func(t *testing.T) {
		publisher.Publish([]stream.Event{
			{
				Topic: pbsubscribe.Topic_ServiceIntentions,
				Index: index + 1,
				Payload: state.EventPayloadConfigEntry{
					Op: pbsubscribe.ConfigEntryUpdate_Upsert,
					Value: &structs.ServiceIntentionsConfigEntry{
						Name: serviceName,
						Sources: []*structs.SourceIntention{
							{Name: "db", Action: structs.IntentionActionAllow, Precedence: 1},
						},
					},
				},
			},
		})

		result := getEventResult[structs.Intentions](t, eventCh)
		require.Len(t, result, 1)

		intention := result[0]
		require.Equal(t, intention.DestinationName, serviceName)
		require.Equal(t, intention.SourceName, "db")
	})

	testutil.RunStep(t, "publishing a wildcard intention", func(t *testing.T) {
		publisher.Publish([]stream.Event{
			{
				Topic: pbsubscribe.Topic_ServiceIntentions,
				Index: index + 2,
				Payload: state.EventPayloadConfigEntry{
					Op: pbsubscribe.ConfigEntryUpdate_Upsert,
					Value: &structs.ServiceIntentionsConfigEntry{
						Name: structs.WildcardSpecifier,
						Sources: []*structs.SourceIntention{
							{Name: structs.WildcardSpecifier, Action: structs.IntentionActionAllow, Precedence: 0},
						},
					},
				},
			},
		})

		result := getEventResult[structs.Intentions](t, eventCh)
		require.Len(t, result, 2)

		a := result[0]
		require.Equal(t, a.DestinationName, serviceName)
		require.Equal(t, a.SourceName, "db")

		b := result[1]
		require.Equal(t, b.DestinationName, structs.WildcardSpecifier)
		require.Equal(t, b.SourceName, structs.WildcardSpecifier)
	})

	testutil.RunStep(t, "publishing a delete event", func(t *testing.T) {
		publisher.Publish([]stream.Event{
			{
				Topic: pbsubscribe.Topic_ServiceIntentions,
				Index: index + 3,
				Payload: state.EventPayloadConfigEntry{
					Op: pbsubscribe.ConfigEntryUpdate_Delete,
					Value: &structs.ServiceIntentionsConfigEntry{
						Name: serviceName,
					},
				},
			},
		})

		result := getEventResult[structs.Intentions](t, eventCh)
		require.Len(t, result, 1)
	})

}

type staticResolver struct {
	mu         sync.Mutex
	authorizer acl.Authorizer
}

func newStaticResolver(authz acl.Authorizer) *staticResolver {
	resolver := new(staticResolver)
	resolver.SwapAuthorizer(authz)
	return resolver
}

func (r *staticResolver) SwapAuthorizer(authz acl.Authorizer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.authorizer = authz
}

func (r *staticResolver) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzContext *acl.AuthorizerContext) (resolver.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return resolver.Result{Authorizer: r.authorizer}, nil
}
