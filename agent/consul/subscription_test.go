package consul

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestConfigEntrySubscriptions(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configEntry func(string) structs.ConfigEntry
		topic       stream.Topic
	}{
		"Subscribe to API Gateway Changes": {
			configEntry: func(name string) structs.ConfigEntry {
				return &structs.APIGatewayConfigEntry{
					Kind: structs.APIGateway,
					Name: name,
				}
			},
			topic: state.EventTopicAPIGateway,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			publisher := stream.NewEventPublisher(1 * time.Millisecond)
			go publisher.Run(ctx)

			store := fsm.NewFromDeps(fsm.Deps{
				Logger: hclog.New(nil),
				NewStateStore: func() *state.Store {
					return state.NewStateStoreWithEventPublisher(nil, publisher)
				},
				Publisher: publisher,
			}).State()

			// Push 200 instances of the config entry to the store.
			for i := 0; i < 200; i++ {
				entryIndex := uint64(i + 1)
				name := fmt.Sprintf("foo-%d", i)
				require.NoError(t, store.EnsureConfigEntry(entryIndex, tc.configEntry(name)))
			}

			received := []string{}

			go func() {
				subscribeRequest := &stream.SubscribeRequest{
					Topic: tc.topic,
				}
				fmt.Println(subscribeRequest) // TODO
			}()

		LOOP:
			for {
				select {
				case <-ctx.Done():
					break LOOP
				}
			}

			require.Len(t, received, 200)
			for i := 0; i < 200; i++ {
				require.Contains(t, received, fmt.Sprintf("foo-%d", i))
			}
		})
	}
}
