// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"context"
)

type CleanerT interface {
	Helper()
	Cleanup(func())
}

func TestContext(t CleanerT) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
