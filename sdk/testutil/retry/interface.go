// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retry

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var nilInf TestingTB = nil

// Assertion that our TestingTB can be passed to
var _ require.TestingT = nilInf
var _ assert.TestingT = nilInf

// TestingTB is an interface that describes the implementation of the testing object.
// Using an interface that describes testing.TB instead of the actual implementation
// makes testutil usable in a wider variety of contexts (e.g. use with ginkgo : https://godoc.org/github.com/onsi/ginkgo#GinkgoT)
type TestingTB interface {
	Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	Name() string
	Setenv(key, value string)
	TempDir() string
}
