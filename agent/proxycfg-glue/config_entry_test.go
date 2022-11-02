package proxycfgglue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbconfigentry"
	"github.com/hashicorp/consul/proto/pbsubscribe"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestConfigEntryView(t *testing.T) {
	const index uint64 = 123

	view := &configEntryView{}

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := view.Result(index)
		resp, ok := result.(*structs.ConfigEntryResponse)
		require.Truef(t, ok, "expected ConfigEntryResponse, got: %T", result)
		require.Nil(t, resp.Entry)
		require.Equal(t, index, resp.QueryMeta.Index)
	})

	testutil.RunStep(t, "upsert event", func(t *testing.T) {
		err := view.Update([]*pbsubscribe.Event{
			{
				Index: index,
				Payload: &pbsubscribe.Event_ConfigEntry{
					ConfigEntry: &pbsubscribe.ConfigEntryUpdate{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						ConfigEntry: &pbconfigentry.ConfigEntry{
							Kind: pbconfigentry.Kind_KindServiceResolver,
							Name: "web",
							Entry: &pbconfigentry.ConfigEntry_ServiceResolver{
								ServiceResolver: &pbconfigentry.ServiceResolver{},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		result := view.Result(index)
		resp, ok := result.(*structs.ConfigEntryResponse)
		require.Truef(t, ok, "expected ConfigEntryResponse, got: %T", result)

		serviceResolver, ok := resp.Entry.(*structs.ServiceResolverConfigEntry)
		require.Truef(t, ok, "expected ServiceResolverConfigEntry, got: %T", resp.Entry)
		require.Equal(t, "web", serviceResolver.Name)
	})

	testutil.RunStep(t, "delete event", func(t *testing.T) {
		err := view.Update([]*pbsubscribe.Event{
			{
				Index: index,
				Payload: &pbsubscribe.Event_ConfigEntry{
					ConfigEntry: &pbsubscribe.ConfigEntryUpdate{
						Op: pbsubscribe.ConfigEntryUpdate_Delete,
						ConfigEntry: &pbconfigentry.ConfigEntry{
							Kind: pbconfigentry.Kind_KindServiceResolver,
							Name: "web",
							Entry: &pbconfigentry.ConfigEntry_ServiceResolver{
								ServiceResolver: &pbconfigentry.ServiceResolver{},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		result := view.Result(index)
		resp, ok := result.(*structs.ConfigEntryResponse)
		require.Truef(t, ok, "expected ConfigEntryResponse, got: %T", result)
		require.Nil(t, resp.Entry)
	})

	testutil.RunStep(t, "bogus event", func(t *testing.T) {
		err := view.Update([]*pbsubscribe.Event{
			{
				Index:   index,
				Payload: &pbsubscribe.Event_ServiceHealth{},
			},
		})
		require.NoError(t, err)

		result := view.Result(index)
		resp, ok := result.(*structs.ConfigEntryResponse)
		require.Truef(t, ok, "expected ConfigEntryResponse, got: %T", result)
		require.Nil(t, resp.Entry)
	})
}

func TestConfigEntryListView(t *testing.T) {
	const index uint64 = 123

	view := newConfigEntryListView(structs.ServiceResolver, *acl.DefaultEnterpriseMeta())

	testutil.RunStep(t, "initial state", func(t *testing.T) {
		result := view.Result(index)

		resp, ok := result.(*structs.IndexedConfigEntries)
		require.Truef(t, ok, "expected IndexedConfigEntries, got: %T", result)
		require.Empty(t, resp.Entries)
		require.Equal(t, index, resp.QueryMeta.Index)
	})

	testutil.RunStep(t, "upsert events", func(t *testing.T) {
		err := view.Update([]*pbsubscribe.Event{
			{
				Index: index,
				Payload: &pbsubscribe.Event_ConfigEntry{
					ConfigEntry: &pbsubscribe.ConfigEntryUpdate{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						ConfigEntry: &pbconfigentry.ConfigEntry{
							Kind: pbconfigentry.Kind_KindServiceResolver,
							Name: "web",
							Entry: &pbconfigentry.ConfigEntry_ServiceResolver{
								ServiceResolver: &pbconfigentry.ServiceResolver{},
							},
						},
					},
				},
			},
			{
				Index: index,
				Payload: &pbsubscribe.Event_ConfigEntry{
					ConfigEntry: &pbsubscribe.ConfigEntryUpdate{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						ConfigEntry: &pbconfigentry.ConfigEntry{
							Kind: pbconfigentry.Kind_KindServiceResolver,
							Name: "db",
							Entry: &pbconfigentry.ConfigEntry_ServiceResolver{
								ServiceResolver: &pbconfigentry.ServiceResolver{},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		result := view.Result(index)
		resp, ok := result.(*structs.IndexedConfigEntries)
		require.Truef(t, ok, "expected IndexedConfigEntries, got: %T", result)
		require.Len(t, resp.Entries, 2)
	})

	testutil.RunStep(t, "delete event", func(t *testing.T) {
		err := view.Update([]*pbsubscribe.Event{
			{
				Index: index,
				Payload: &pbsubscribe.Event_ConfigEntry{
					ConfigEntry: &pbsubscribe.ConfigEntryUpdate{
						Op: pbsubscribe.ConfigEntryUpdate_Delete,
						ConfigEntry: &pbconfigentry.ConfigEntry{
							Kind: pbconfigentry.Kind_KindServiceResolver,
							Name: "web",
							Entry: &pbconfigentry.ConfigEntry_ServiceResolver{
								ServiceResolver: &pbconfigentry.ServiceResolver{},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		result := view.Result(index)
		resp, ok := result.(*structs.IndexedConfigEntries)
		require.Truef(t, ok, "expected IndexedConfigEntries, got: %T", result)
		require.Len(t, resp.Entries, 1)

		serviceResolver, ok := resp.Entries[0].(*structs.ServiceResolverConfigEntry)
		require.Truef(t, ok, "expected ServiceResolverConfigEntry, got: %T", resp.Entries[0])
		require.Equal(t, "db", serviceResolver.Name)
	})
}
