package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestIndexConnectService_FromObject(t *testing.T) {
	cases := []struct {
		Name        string
		Input       interface{}
		ExpectMatch bool
		ExpectVal   []byte
		ExpectErr   string
	}{
		{
			"not a ServiceNode",
			42,
			false,
			nil,
			"ServiceNode",
		},

		{
			"typical service, not native",
			&structs.ServiceNode{
				ServiceName: "db",
			},
			false,
			nil,
			"",
		},

		{
			"typical service, is native",
			&structs.ServiceNode{
				ServiceName:    "dB",
				ServiceConnect: structs.ServiceConnect{Native: true},
			},
			true,
			[]byte("db\x00"),
			"",
		},

		{
			"proxy service",
			&structs.ServiceNode{
				ServiceKind:             structs.ServiceKindConnectProxy,
				ServiceName:             "db",
				ServiceProxyDestination: "fOo",
			},
			true,
			[]byte("foo\x00"),
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			var idx IndexConnectService
			match, val, err := idx.FromObject(tc.Input)
			if tc.ExpectErr != "" {
				require.Error(err)
				require.Contains(err.Error(), tc.ExpectErr)
				return
			}
			require.NoError(err)
			require.Equal(tc.ExpectMatch, match)
			require.Equal(tc.ExpectVal, val)
		})
	}
}

func TestIndexConnectService_FromArgs(t *testing.T) {
	cases := []struct {
		Name      string
		Args      []interface{}
		ExpectVal []byte
		ExpectErr string
	}{
		{
			"multiple arguments",
			[]interface{}{"foo", "bar"},
			nil,
			"single",
		},

		{
			"not a string",
			[]interface{}{42},
			nil,
			"must be a string",
		},

		{
			"string",
			[]interface{}{"fOO"},
			[]byte("foo\x00"),
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			var idx IndexConnectService
			val, err := idx.FromArgs(tc.Args...)
			if tc.ExpectErr != "" {
				require.Error(err)
				require.Contains(err.Error(), tc.ExpectErr)
				return
			}
			require.NoError(err)
			require.Equal(tc.ExpectVal, val)
		})
	}
}
