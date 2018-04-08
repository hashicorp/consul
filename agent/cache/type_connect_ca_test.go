package cache

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTypeCARoot(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &TypeCARoot{RPC: rpc}

	// Expect the proper RPC call. This also sets the expected value
	// since that is return-by-pointer in the arguments.
	var resp *structs.IndexedCARoots
	rpc.On("RPC", "ConnectCA.Roots", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*structs.DCSpecificRequest)
			require.Equal(uint64(24), req.QueryOptions.MinQueryIndex)
			require.Equal(1*time.Second, req.QueryOptions.MaxQueryTime)

			reply := args.Get(2).(*structs.IndexedCARoots)
			reply.QueryMeta.Index = 48
			resp = reply
		})

	// Fetch
	result, err := typ.Fetch(FetchOptions{
		MinIndex: 24,
		Timeout:  1 * time.Second,
	}, &structs.DCSpecificRequest{Datacenter: "dc1"})
	require.Nil(err)
	require.Equal(FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestTypeCARoot_badReqType(t *testing.T) {
	require := require.New(t)
	rpc := TestRPC(t)
	defer rpc.AssertExpectations(t)
	typ := &TypeCARoot{RPC: rpc}

	// Fetch
	_, err := typ.Fetch(FetchOptions{}, TestRequest(t, "foo", 64))
	require.NotNil(err)
	require.Contains(err.Error(), "wrong type")

}
