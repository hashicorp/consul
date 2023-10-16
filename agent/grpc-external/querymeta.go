// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package external

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/agent/structs"
)

func StringToQueryBackendDecodeHookFunc(f reflect.Type, t reflect.Type, data any) (any, error) {
	if f.Kind() != reflect.String {
		return data, nil
	}
	if t != reflect.TypeOf(structs.QueryBackend(0)) {
		return data, nil
	}

	name, ok := data.(string)
	if !ok {
		return data, fmt.Errorf("could not parse query backend as string")
	}

	return structs.QueryBackendFromString(name), nil
}

// QueryMetaFromGRPCMeta returns a structs.QueryMeta struct parsed from the metadata.MD,
// such as from a gRPC header or trailer.
func QueryMetaFromGRPCMeta(md metadata.MD) (structs.QueryMeta, error) {
	var queryMeta structs.QueryMeta

	m := map[string]string{}
	for k, v := range md {
		m[k] = v[0]
	}

	decodeHooks := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		StringToQueryBackendDecodeHookFunc,
	)

	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &queryMeta,
		WeaklyTypedInput: true,
		DecodeHook:       decodeHooks,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return queryMeta, err
	}

	err = decoder.Decode(m)
	if err != nil {
		return queryMeta, err
	}

	return queryMeta, nil
}

// GRPCMetadataFromQueryMeta returns a metadata struct with fields from the structs.QueryMeta attached.
// The return value is suitable for attaching to a gRPC header/trailer.
func GRPCMetadataFromQueryMeta(queryMeta structs.QueryMeta) (metadata.MD, error) {
	md := metadata.MD{}
	m := map[string]any{}
	err := mapstructure.Decode(queryMeta, &m)
	if err != nil {
		return nil, err
	}
	for k, v := range m {
		md.Set(k, fmt.Sprintf("%v", v))
	}
	return md, nil
}
