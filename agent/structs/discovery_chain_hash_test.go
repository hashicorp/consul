// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestHashCoverage_AllGetHashStructsIncludeAllFields(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "CompiledDiscoveryChain", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[CompiledDiscoveryChain](t, (*CompiledDiscoveryChain).GetHash)
		}},
		{name: "ServiceRoute", run: func(t *testing.T) { requireHashChangesWhenAnyFieldChanges[ServiceRoute](t, (*ServiceRoute).getHash) }},
		{name: "ServiceRouteMatch", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceRouteMatch](t, (*ServiceRouteMatch).getHash)
		}},
		{name: "ServiceRouteHTTPMatch", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceRouteHTTPMatch](t, (*ServiceRouteHTTPMatch).getHash)
		}},
		{name: "ServiceRouteHTTPMatchHeader", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceRouteHTTPMatchHeader](t, (*ServiceRouteHTTPMatchHeader).getHash)
		}},
		{name: "ServiceRouteHTTPMatchQueryParam", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceRouteHTTPMatchQueryParam](t, (*ServiceRouteHTTPMatchQueryParam).getHash)
		}},
		{name: "ServiceRouteDestination", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceRouteDestination](t, (*ServiceRouteDestination).getHash)
		}},
		{name: "ServiceSplit", run: func(t *testing.T) { requireHashChangesWhenAnyFieldChanges[ServiceSplit](t, (*ServiceSplit).getHash) }},
		{name: "ServiceResolverSubset", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceResolverSubset](t, (*ServiceResolverSubset).getHash)
		}},
		{name: "ServiceResolverFailoverPolicy", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[ServiceResolverFailoverPolicy](t, (*ServiceResolverFailoverPolicy).getHash)
		}},
		{name: "LoadBalancer", run: func(t *testing.T) { requireHashChangesWhenAnyFieldChanges[LoadBalancer](t, (*LoadBalancer).getHash) }},
		{name: "RingHashConfig", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[RingHashConfig](t, (*RingHashConfig).getHash)
		}},
		{name: "LeastRequestConfig", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[LeastRequestConfig](t, (*LeastRequestConfig).getHash)
		}},
		{name: "HashPolicy", run: func(t *testing.T) { requireHashChangesWhenAnyFieldChanges[HashPolicy](t, (*HashPolicy).getHash) }},
		{name: "CookieConfig", run: func(t *testing.T) { requireHashChangesWhenAnyFieldChanges[CookieConfig](t, (*CookieConfig).getHash) }},
		{name: "HTTPHeaderModifiers", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[HTTPHeaderModifiers](t, (*HTTPHeaderModifiers).getHash)
		}},
		{name: "EnvoyExtension", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[EnvoyExtension](t, (*EnvoyExtension).getHash)
		}},
		{name: "MeshGatewayConfig", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[MeshGatewayConfig](t, (*MeshGatewayConfig).getHash)
		}},
		{name: "TransparentProxyConfig", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[TransparentProxyConfig](t, (*TransparentProxyConfig).getHash)
		}},
		{name: "DiscoveryGraphNode", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoveryGraphNode](t, (*DiscoveryGraphNode).getHash)
		}},
		{name: "DiscoveryResolver", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoveryResolver](t, (*DiscoveryResolver).getHash)
		}},
		{name: "DiscoveryRoute", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoveryRoute](t, (*DiscoveryRoute).getHash)
		}},
		{name: "DiscoverySplit", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoverySplit](t, (*DiscoverySplit).getHash)
		}},
		{name: "DiscoveryFailover", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoveryFailover](t, (*DiscoveryFailover).getHash)
		}},
		{name: "DiscoveryPrioritizeByLocality", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoveryPrioritizeByLocality](t, (*DiscoveryPrioritizeByLocality).getHash)
		}},
		{name: "DiscoveryTarget", run: func(t *testing.T) {
			requireHashChangesWhenAnyFieldChanges[DiscoveryTarget](t, (*DiscoveryTarget).getHash)
		}},
		{name: "Locality", run: func(t *testing.T) { requireHashChangesWhenAnyFieldChanges[Locality](t, (*Locality).getHash) }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, testCase.run)
	}
}

func requireHashChangesWhenAnyFieldChanges[T any](t *testing.T, getHash func(*T) uint64) {
	t.Helper()

	base := new(T)
	typeOf := reflect.TypeOf(*base)
	require.Equal(t, reflect.Struct, typeOf.Kind(), "requireHashChangesWhenAnyFieldChanges requires a struct type")

	baseHash := getHash(base)
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		mutated := new(T)

		err := populateNonZeroReflectValue(settableValue(reflect.ValueOf(mutated).Elem().Field(i)), field.Name)
		require.NoError(t, err, "failed to populate field %s; extend the hash coverage helper for this type", field.Name)
		require.NotEqual(t, baseHash, getHash(mutated), "field %s changed without affecting %s hash; update appendHash/getHash coverage", field.Name, typeOf.Name())
	}
}

func settableValue(value reflect.Value) reflect.Value {
	if value.CanSet() {
		return value
	}
	return reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem()
}

func populateNonZeroReflectValue(value reflect.Value, label string) error {
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(true)
		return nil
	case reflect.String:
		value.SetString(label)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(1)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value.SetUint(1)
		return nil
	case reflect.Float32, reflect.Float64:
		value.SetFloat(1)
		return nil
	case reflect.Pointer:
		elem := reflect.New(value.Type().Elem())
		if err := populateNonZeroReflectValue(elem.Elem(), label); err != nil {
			return err
		}
		value.Set(elem)
		return nil
	case reflect.Interface:
		value.Set(reflect.ValueOf(label))
		return nil
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if !value.Field(i).CanAddr() {
				continue
			}
			field := value.Type().Field(i)
			if err := populateNonZeroReflectValue(settableValue(value.Field(i)), field.Name); err == nil {
				return nil
			}
		}
		return fmt.Errorf("struct %s has no supported fields", value.Type())
	case reflect.Slice:
		slice := reflect.MakeSlice(value.Type(), 1, 1)
		if err := populateNonZeroReflectValue(settableValue(slice.Index(0)), label); err != nil {
			return err
		}
		value.Set(slice)
		return nil
	case reflect.Map:
		key := reflect.New(value.Type().Key()).Elem()
		if err := populateNonZeroReflectValue(settableValue(key), label+"Key"); err != nil {
			return err
		}
		mapValue := reflect.New(value.Type().Elem()).Elem()
		if err := populateNonZeroReflectValue(settableValue(mapValue), label+"Value"); err != nil {
			return err
		}
		m := reflect.MakeMapWithSize(value.Type(), 1)
		m.SetMapIndex(key, mapValue)
		value.Set(m)
		return nil
	default:
		return fmt.Errorf("unsupported kind %s for %s", value.Kind(), label)
	}
}
