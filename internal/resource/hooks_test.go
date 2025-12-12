// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/stretchr/testify/require"
)

func TestDecodeAndValidate(t *testing.T) {
	res := rtest.Resource(demo.TypeV2Artist, "babypants").
		WithData(t, &pbdemo.Artist{Name: "caspar babypants"}).
		Build()

	t.Run("ok", func(t *testing.T) {
		err := resource.DecodeAndValidate[*pbdemo.Artist](func(dec *resource.DecodedResource[*pbdemo.Artist]) error {
			require.NotNil(t, dec.Resource)
			require.NotNil(t, dec.Data)

			return nil
		})(res)

		require.NoError(t, err)
	})

	t.Run("inner-validation-error", func(t *testing.T) {
		fakeErr := fmt.Errorf("fake")

		err := resource.DecodeAndValidate[*pbdemo.Artist](func(dec *resource.DecodedResource[*pbdemo.Artist]) error {
			return fakeErr
		})(res)

		require.Error(t, err)
		require.Equal(t, fakeErr, err)
	})

	t.Run("decode-error", func(t *testing.T) {
		err := resource.DecodeAndValidate[*pbdemo.Album](func(dec *resource.DecodedResource[*pbdemo.Album]) error {
			require.Fail(t, "callback should not be called when decoding fails")
			return nil
		})(res)

		require.Error(t, err)
		require.ErrorAs(t, err, &resource.ErrDataParse{})
	})
}

func TestDecodeAndMutate(t *testing.T) {
	res := rtest.Resource(demo.TypeV2Artist, "babypants").
		WithData(t, &pbdemo.Artist{Name: "caspar babypants"}).
		Build()

	t.Run("no-writeback", func(t *testing.T) {
		original := res.Data.Value

		err := resource.DecodeAndMutate[*pbdemo.Artist](func(dec *resource.DecodedResource[*pbdemo.Artist]) (bool, error) {
			require.NotNil(t, dec.Resource)
			require.NotNil(t, dec.Data)

			// we are going to change the data but not tell the outer hook about it
			dec.Data.Name = "changed"

			return false, nil
		})(res)

		require.NoError(t, err)
		// Ensure that the outer hook didn't overwrite the resources data because we told it not to
		require.Equal(t, original, res.Data.Value)
	})

	t.Run("writeback", func(t *testing.T) {
		original := res.Data.Value

		err := resource.DecodeAndMutate[*pbdemo.Artist](func(dec *resource.DecodedResource[*pbdemo.Artist]) (bool, error) {
			require.NotNil(t, dec.Resource)
			require.NotNil(t, dec.Data)

			dec.Data.Name = "changed"

			return true, nil
		})(res)

		require.NoError(t, err)
		// Ensure that the outer hook reencoded the Any data because we told it to.
		require.NotEqual(t, original, res.Data.Value)
	})

	t.Run("inner-mutation-error", func(t *testing.T) {
		fakeErr := fmt.Errorf("fake")

		err := resource.DecodeAndMutate[*pbdemo.Artist](func(dec *resource.DecodedResource[*pbdemo.Artist]) (bool, error) {
			return false, fakeErr
		})(res)

		require.Error(t, err)
		require.Equal(t, fakeErr, err)
	})

	t.Run("decode-error", func(t *testing.T) {
		err := resource.DecodeAndMutate[*pbdemo.Album](func(dec *resource.DecodedResource[*pbdemo.Album]) (bool, error) {
			require.Fail(t, "callback should not be called when decoding fails")
			return false, nil
		})(res)

		require.Error(t, err)
		require.ErrorAs(t, err, &resource.ErrDataParse{})
	})
}

func TestDecodeAndAuthorizeWrite(t *testing.T) {
	res := rtest.Resource(demo.TypeV2Artist, "babypants").
		WithData(t, &pbdemo.Artist{Name: "caspar babypants"}).
		Build()

	t.Run("allowed", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeWrite[*pbdemo.Artist](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Artist]) error {
			require.NotNil(t, a)
			require.NotNil(t, c)
			require.NotNil(t, dec.Resource)
			require.NotNil(t, dec.Data)

			// access allowed
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, res)

		require.NoError(t, err)
	})

	t.Run("denied", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeWrite[*pbdemo.Artist](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Artist]) error {
			return acl.PermissionDenied("fake")
		})(acl.DenyAll(), nil, res)

		require.Error(t, err)
		require.True(t, acl.IsErrPermissionDenied(err))
	})

	t.Run("decode-error", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeWrite[*pbdemo.Album](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Album]) error {
			require.Fail(t, "callback should not be called when decoding fails")
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, res)

		require.Error(t, err)
		require.ErrorAs(t, err, &resource.ErrDataParse{})
	})
}

func TestDecodeAndAuthorizeRead(t *testing.T) {
	res := rtest.Resource(demo.TypeV2Artist, "babypants").
		WithData(t, &pbdemo.Artist{Name: "caspar babypants"}).
		Build()

	t.Run("allowed", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeRead[*pbdemo.Artist](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Artist]) error {
			require.NotNil(t, a)
			require.NotNil(t, c)
			require.NotNil(t, dec.Resource)
			require.NotNil(t, dec.Data)

			// access allowed
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, nil, res)

		require.NoError(t, err)
	})

	t.Run("denied", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeRead[*pbdemo.Artist](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Artist]) error {
			return acl.PermissionDenied("fake")
		})(acl.DenyAll(), nil, nil, res)

		require.Error(t, err)
		require.True(t, acl.IsErrPermissionDenied(err))
	})

	t.Run("decode-error", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeRead[*pbdemo.Album](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Album]) error {
			require.Fail(t, "callback should not be called when decoding fails")
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, nil, res)

		require.Error(t, err)
		require.ErrorAs(t, err, &resource.ErrDataParse{})
	})

	t.Run("err-need-resource", func(t *testing.T) {
		err := resource.DecodeAndAuthorizeRead[*pbdemo.Artist](func(a acl.Authorizer, c *acl.AuthorizerContext, dec *resource.DecodedResource[*pbdemo.Artist]) error {
			require.Fail(t, "callback should not be called when no resource was provided to be decoded")
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, nil, nil)

		require.Error(t, err)
		require.ErrorIs(t, err, resource.ErrNeedResource)
	})
}

func TestAuthorizeReadWithResource(t *testing.T) {
	res := rtest.Resource(demo.TypeV2Artist, "babypants").
		WithData(t, &pbdemo.Artist{Name: "caspar babypants"}).
		Build()

	t.Run("allowed", func(t *testing.T) {
		err := resource.AuthorizeReadWithResource(func(a acl.Authorizer, c *acl.AuthorizerContext, res *pbresource.Resource) error {
			require.NotNil(t, a)
			require.NotNil(t, c)
			require.NotNil(t, res)

			// access allowed
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, nil, res)

		require.NoError(t, err)
	})

	t.Run("denied", func(t *testing.T) {
		err := resource.AuthorizeReadWithResource(func(a acl.Authorizer, c *acl.AuthorizerContext, res *pbresource.Resource) error {
			return acl.PermissionDenied("fake")
		})(acl.DenyAll(), nil, nil, res)

		require.Error(t, err)
		require.True(t, acl.IsErrPermissionDenied(err))
	})

	t.Run("err-need-resource", func(t *testing.T) {
		err := resource.AuthorizeReadWithResource(func(a acl.Authorizer, c *acl.AuthorizerContext, res *pbresource.Resource) error {
			require.Fail(t, "callback should not be called when no resource was provided to be decoded")
			return nil
		})(acl.DenyAll(), &acl.AuthorizerContext{}, nil, nil)

		require.Error(t, err)
		require.ErrorIs(t, err, resource.ErrNeedResource)
	})
}
