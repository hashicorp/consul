// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cachetype

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/cacheshim"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const ConnectCARootName = cacheshim.ConnectCARootName

// ConnectCARoot supports fetching the Connect CA roots. This is a
// straightforward cache type since it only has to block on the given
// index and return the data.
type ConnectCARoot struct {
	RegisterOptionsBlockingRefresh
	RPC RPC
}

func (c *ConnectCARoot) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {

	fmt.Println(time.Now().String() + " ===================>  ConnectCARoot Fetch function called")
	// debug.PrintStack()
	var result cache.FetchResult

	// The request should be a DCSpecificRequest.
	reqReal, ok := req.(*structs.DCSpecificRequest)

	if !ok {
		fmt.Println(time.Now().String() + " ===================>  ConnectCARoot Fetch function called 1")

		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}
	b, err := json.Marshal(reqReal)
	if err != nil {
		fmt.Println(time.Now().String()+" ===================>  ConnectCARoot Fetch function called 3", err)

	}
	fmt.Println(time.Now().String()+" ===================>  ConnectCARoot Fetch function called 4", string(b))

	// Lightweight copy this object so that manipulating QueryOptions doesn't race.
	dup := *reqReal
	reqReal = &dup

	// Set the minimum query index to our current index so we block
	reqReal.MinQueryIndex = opts.MinIndex
	reqReal.MaxQueryTime = opts.Timeout
	reqReal.Token = "" // No ACL token needed to fetch Connect CA roots

	var reply structs.IndexedCARoots
	if err := c.RPC.RPC(context.Background(), "ConnectCA.Roots", reqReal, &reply); err != nil {
		fmt.Println(time.Now().String() + " ===================>  ConnectCARoot Fetch function called 5")

		return result, err
	}
	fmt.Println(time.Now().String() + " ===================>  ConnectCARoot Fetch function called 6")

	result.Value = &reply
	result.Index = reply.Index
	return result, nil
}
