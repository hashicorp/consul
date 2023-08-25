// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package protohcl

import (
	"encoding/base64"
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func boolFromCty(val cty.Value) (bool, error) {
	if val.Type() != cty.Bool {
		return false, fmt.Errorf("expected value of type %s but actual type is %s", cty.Bool.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return false, nil
	}

	return val.True(), nil
}

func int32FromCty(val cty.Value) (int32, error) {
	if val.Type() != cty.Number {
		return 0, fmt.Errorf("expected value of type %s but actual type is %s", cty.Number.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return 0, nil
	}

	var goVal int32
	if err := gocty.FromCtyValue(val, &goVal); err != nil {
		return 0, fmt.Errorf("error converting cty value of type %s to int32: %w", val.Type().FriendlyName(), err)
	}
	return goVal, nil
}

func uint32FromCty(val cty.Value) (uint32, error) {
	if val.Type() != cty.Number {
		return 0, fmt.Errorf("expected value of type %s but actual type is %s", cty.Number.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return 0, nil
	}

	var goVal uint32
	if err := gocty.FromCtyValue(val, &goVal); err != nil {
		return 0, fmt.Errorf("error converting cty value of type %s to uint32: %w", val.Type().FriendlyName(), err)
	}
	return goVal, nil
}

func int64FromCty(val cty.Value) (int64, error) {
	if val.Type() != cty.Number {
		return 0, fmt.Errorf("expected value of type %s but actual type is %s", cty.Number.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return 0, nil
	}

	var goVal int64
	if err := gocty.FromCtyValue(val, &goVal); err != nil {
		return 0, fmt.Errorf("error converting cty value of type %s to int64: %w", val.Type().FriendlyName(), err)
	}
	return goVal, nil
}

func uint64FromCty(val cty.Value) (uint64, error) {
	if val.Type() != cty.Number {
		return 0, fmt.Errorf("expected value of type %s but actual type is %s", cty.Number.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return 0, nil
	}

	var goVal uint64
	if err := gocty.FromCtyValue(val, &goVal); err != nil {
		return 0, fmt.Errorf("error converting cty value of type %s to uint64: %w", val.Type().FriendlyName(), err)
	}
	return goVal, nil
}

func floatFromCty(val cty.Value) (float32, error) {
	if val.Type() != cty.Number {
		return 0, fmt.Errorf("expected value of type %s but actual type is %s", cty.Number.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return 0, nil
	}

	var goVal float32
	if err := gocty.FromCtyValue(val, &goVal); err != nil {
		return 0, fmt.Errorf("error converting cty value of type %s to float32: %w", val.Type().FriendlyName(), err)
	}
	return goVal, nil
}

func doubleFromCty(val cty.Value) (float64, error) {
	if val.Type() != cty.Number {
		return 0, fmt.Errorf("expected value of type %s but actual type is %s", cty.Number.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return 0, nil
	}

	var goVal float64
	if err := gocty.FromCtyValue(val, &goVal); err != nil {
		return 0, fmt.Errorf("error converting cty value of type %s to float64: %w", val.Type().FriendlyName(), err)
	}
	return goVal, nil
}

func stringFromCty(val cty.Value) (string, error) {
	if val.Type() != cty.String {
		return "", fmt.Errorf("expected value of type %s but actual type is %s", cty.String.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return "", nil
	}
	return val.AsString(), nil
}

func bytesFromCty(val cty.Value) ([]byte, error) {
	if val.Type() != cty.String {
		return nil, fmt.Errorf("expected value of type %s but actual type is %s", cty.String.FriendlyName(), val.Type().FriendlyName())
	}

	if val.IsNull() {
		return nil, nil
	}

	encoded := val.AsString()
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding byte string: %w", err)
	}

	return decoded, nil
}
