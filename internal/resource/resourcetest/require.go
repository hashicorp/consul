// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

// CompareErrorString is a helper to generate a custom go-cmp comparer method
// that will perform an equality check on the error message. This is mainly
// useful to get around not being able to see unexported data within errors.
func CompareErrorString[T error]() cmp.Option {
	return cmp.Comparer(func(e1, e2 T) bool {
		return e1.Error() == e2.Error()
	})
}

// default comparers for known types that don't play well with go-cmp
var comparers = []cmp.Option{
	CompareErrorString[resource.ConstError](),
}

// RequireError is useful for asserting that some chained multierror contains a specific error.
func RequireError[E error](t T, err error, expected E, opts ...cmp.Option) {
	t.Helper()

	var actual E
	require.ErrorAs(t, err, &actual)

	opts = append(opts, comparers...)
	prototest.AssertDeepEqual(t, expected, actual, opts...)
}

func RequireVersionUnchanged(t T, res *pbresource.Resource, version string) {
	t.Helper()
	require.Equal(t, version, res.Version)
}

func RequireVersionChanged(t T, res *pbresource.Resource, version string) {
	t.Helper()
	require.NotEqual(t, version, res.Version)
}

func RequireStatusCondition(t T, res *pbresource.Resource, statusKey string, condition *pbresource.Condition) {
	t.Helper()
	require.NotNil(t, res.Status)
	status, found := res.Status[statusKey]
	require.True(t, found)
	prototest.AssertContainsElement(t, status.Conditions, condition)
}

func RequireStatusConditionForCurrentGen(t T, res *pbresource.Resource, statusKey string, condition *pbresource.Condition) {
	t.Helper()
	require.NotNil(t, res.Status)
	status, found := res.Status[statusKey]
	require.True(t, found)
	require.Equal(t, res.Generation, status.ObservedGeneration)
	prototest.AssertContainsElement(t, status.Conditions, condition)
}

func RequireResourceMeta(t T, res *pbresource.Resource, key string, value string) {
	t.Helper()
	require.NotNil(t, res.Metadata)
	require.Contains(t, res.Metadata, key)
	require.Equal(t, value, res.Metadata[key])
}

func RequireReconciledCurrentGen(t T, res *pbresource.Resource, statusKey string) {
	t.Helper()
	require.NotNil(t, res.Status)
	status, found := res.Status[statusKey]
	require.True(t, found)
	require.Equal(t, res.Generation, status.ObservedGeneration)
}

func RequireOwner(t T, res *pbresource.Resource, owner *pbresource.ID, ignoreUid bool) {
	t.Helper()

	var opts []cmp.Option
	if ignoreUid {
		opts = append(opts, protocmp.IgnoreFields(owner, "uid"))
	}

	prototest.AssertDeepEqual(t, res.Owner, owner, opts...)
}
