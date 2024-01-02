// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cachetype

import "context"

// RPC is an interface that an RPC client must implement. This is a helper
// interface that is implemented by the agent delegate so that Type
// implementations can request RPC access.
//
//go:generate mockery --name RPC --inpackage
type RPC interface {
	RPC(ctx context.Context, method string, args interface{}, reply interface{}) error
}
