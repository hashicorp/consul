package connect

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
)

func TestSpiffeIDServiceAuthorize(t *testing.T) {
	ns := structs.IntentionDefaultNamespace
	serviceWeb := &SpiffeIDService{
		Host:       "1234.consul",
		Namespace:  structs.IntentionDefaultNamespace,
		Datacenter: "dc01",
		Service:    "web",
	}

	cases := []struct {
		Name  string
		URI   *SpiffeIDService
		Ixn   *structs.Intention
		Auth  bool
		Match bool
	}{
		{
			"exact source, not matching namespace",
			serviceWeb,
			&structs.Intention{
				SourceNS:   "different",
				SourceName: "db",
			},
			false,
			false,
		},

		{
			"exact source, not matching name",
			serviceWeb,
			&structs.Intention{
				SourceNS:   ns,
				SourceName: "db",
			},
			false,
			false,
		},

		{
			"exact source, allow",
			serviceWeb,
			&structs.Intention{
				SourceNS:   serviceWeb.Namespace,
				SourceName: serviceWeb.Service,
				Action:     structs.IntentionActionAllow,
			},
			true,
			true,
		},

		{
			"exact source, deny",
			serviceWeb,
			&structs.Intention{
				SourceNS:   serviceWeb.Namespace,
				SourceName: serviceWeb.Service,
				Action:     structs.IntentionActionDeny,
			},
			false,
			true,
		},

		{
			"exact namespace, wildcard service, deny",
			serviceWeb,
			&structs.Intention{
				SourceNS:   serviceWeb.Namespace,
				SourceName: structs.IntentionWildcard,
				Action:     structs.IntentionActionDeny,
			},
			false,
			true,
		},

		{
			"exact namespace, wildcard service, allow",
			serviceWeb,
			&structs.Intention{
				SourceNS:   serviceWeb.Namespace,
				SourceName: structs.IntentionWildcard,
				Action:     structs.IntentionActionAllow,
			},
			true,
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			auth, match := tc.URI.Authorize(tc.Ixn)
			assert.Equal(t, tc.Auth, auth)
			assert.Equal(t, tc.Match, match)
		})
	}
}
