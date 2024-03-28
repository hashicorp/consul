// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/metadata"
)

// Context is used augment a DNS message with Consul-specific metadata.
type Context struct {
	Token            string `mapstructure:"x-consul-token,omitempty"`
	DefaultNamespace string `mapstructure:"x-consul-namespace,omitempty"`
	DefaultPartition string `mapstructure:"x-consul-partition,omitempty"`
}

// NewContextFromGRPCContext returns the request context using the gRPC metadata attached to the
// given context. If there is no gRPC metadata, it returns an empty context.
func NewContextFromGRPCContext(ctx context.Context) (Context, error) {
	if ctx == nil {
		return Context{}, nil
	}

	reqCtx := Context{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return reqCtx, nil
	}

	m := map[string]string{}
	for k, v := range md {
		m[k] = v[0]
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &reqCtx,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return Context{}, fmt.Errorf("error creating mapstructure decoder: %w", err)
	}

	err = decoder.Decode(m)
	if err != nil {
		return Context{}, fmt.Errorf("error decoding metadata: %w", err)
	}

	return reqCtx, nil
}
