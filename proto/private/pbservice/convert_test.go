// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pbservice

import (
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	fuzz "github.com/google/gofuzz"

	"github.com/hashicorp/consul/agent/structs"
)

func TestNewCheckServiceNodeFromStructs_RoundTrip(t *testing.T) {
	repeat(t, func(t *testing.T, fuzzer *fuzz.Fuzzer) {
		fuzzer.Funcs(randInt32, randUint32, randInterface, randStructsUpstream, randEnterpriseMeta, randStructsConnectProxyConfig)
		var target structs.CheckServiceNode
		fuzzer.Fuzz(&target)

		result, err := CheckServiceNodeToStructs(NewCheckServiceNodeFromStructs(&target))
		if err != nil {
			t.Fatalf("unexpected error: %s", err.Error())
		}
		assertEqual(t, &target, result)
	})
}

func repeat(t *testing.T, fn func(t *testing.T, fuzzer *fuzz.Fuzzer)) {
	reps := getEnvIntWithDefault(t, "TEST_REPEAT_COUNT", 5)
	seed := getEnvIntWithDefault(t, "TEST_RANDOM_SEED", time.Now().UnixNano())
	t.Logf("using seed %d for %d repetitions", seed, reps)

	fuzzer := fuzz.NewWithSeed(seed)
	for i := 0; i < int(reps); i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			fn(t, fuzzer)
		})
	}
}

func getEnvIntWithDefault(t *testing.T, key string, d int64) int64 {
	t.Helper()
	raw, ok := os.LookupEnv(key)
	if !ok {
		return d
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("invald value for %v: %v", key, err.Error())
	}
	return int64(v)
}

func assertEqual(t *testing.T, x, y interface{}) {
	t.Helper()
	if diff := cmp.Diff(x, y, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- original\n+++ result\n\n%v", diff)
	}
}

// randUint32 is a custom fuzzer function which limits all uints to 32 bits.
// This is necessary because the structs types use un-sized uints, however in
// practice they are constrained to 32 bits, and the protobuf types use (u)int32.
// The structs types use (u)int64 for any fields that require 64 bits.
func randUint32(i *uint, c fuzz.Continue) {
	*i = uint(c.Rand.Uint32())
}

// see randUint32
func randInt32(i *int, c fuzz.Continue) {
	*i = int(c.Rand.Int31())
}

// randStructsConnectProxyConfig is a custom fuzzer function which skips
// generating values for fields enumerated in the ignore-fields annotation.
func randStructsConnectProxyConfig(p *structs.ConnectProxyConfig, c fuzz.Continue) {
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "MutualTLSMode":
			continue
		}
		c.Fuzz(v.Field(i).Addr().Interface())
	}
}

// randStructsUpstream is a custom fuzzer function which skips generating values
// for fields enumerated in the ignore-fields annotation.
func randStructsUpstream(u *structs.Upstream, c fuzz.Continue) {
	v := reflect.ValueOf(u).Elem()
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "IngressHosts":
			continue
		}
		c.Fuzz(v.Field(i).Addr().Interface())
	}
}

// randInterface is a custom fuzzer function which generates random data for
// interface{} (most likely used in a map[string]interface{}).
// The random data does not contain any ints (or float32) because protobuf
// converts them to float64, which will cause the test to fail.
func randInterface(m *interface{}, c fuzz.Continue) {
	switch c.Rand.Intn(6) {
	case 0:
		*m = nil
	case 1:
		*m = c.RandBool()
	case 2:
		*m = c.Rand.Float64()
	case 3:
		*m = c.RandString()
	case 4:
		*m = []interface{}{c.RandString(), c.RandBool(), nil, c.Rand.Float64()}
	case 5:
		*m = map[string]interface{}{
			c.RandString(): c.RandString(),
			c.RandString(): c.Rand.Float64(),
			c.RandString(): nil,
		}
	}
}
