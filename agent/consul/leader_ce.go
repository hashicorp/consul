// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"context"

	"github.com/hashicorp/consul/internal/storage"
)

func (s *Server) createDefaultPartition(ctx context.Context, b storage.Backend) error {
	// no-op
	return nil
}
