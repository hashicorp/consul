// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestIsValidDNSLabel(t *testing.T) {
	type testCase struct {
		name  string
		valid bool
	}

	cases := map[string]testCase{
		"min-length": {
			name:  "a",
			valid: true,
		},
		"max-length": {
			name:  strings.Repeat("a1b2c3d4", 8),
			valid: true,
		},
		"underscores": {
			name:  "has_underscores",
			valid: true,
		},
		"hyphenated": {
			name:  "has-hyphen3",
			valid: true,
		},
		"uppercase-not-allowed": {
			name:  "UPPERCASE",
			valid: false,
		},
		"numeric-start": {
			name:  "1abc",
			valid: true,
		},
		"fully-numeric": {
			name:  "1234",
			valid: true,
		},
		"underscore-start-not-allowed": {
			name:  "_abc",
			valid: false,
		},
		"hyphen-start-not-allowed": {
			name:  "-abc",
			valid: false,
		},
		"underscore-end-not-allowed": {
			name:  "abc_",
			valid: false,
		},
		"hyphen-end-not-allowed": {
			name:  "abc-",
			valid: false,
		},
		"unicode-not allowed": {
			name:  "abc∑",
			valid: false,
		},
		"too-long": {
			name:  strings.Repeat("aaaaaaaaa", 8),
			valid: false,
		},
		"missing-name": {
			name:  "",
			valid: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.valid, isValidDNSLabel(tcase.name))
		})
	}
}

func TestIsValidDNSName(t *testing.T) {
	// TestIsValidDNSLabel tests all of the individual label matching
	// criteria. This test therefore is more limited to just the extra
	// validations that IsValidDNSName does. Mainly that it ensures
	// the overall length is less than 256 and that generally is made
	// up of DNS labels joined with '.'

	type testCase struct {
		name  string
		valid bool
	}

	cases := map[string]testCase{
		"valid": {
			name:  "foo-something.example3.com",
			valid: true,
		},
		"exceeds-max-length": {
			name:  strings.Repeat("aaaa.aaaa", 29),
			valid: false,
		},
		"invalid-label": {
			name:  "foo._bar.com",
			valid: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.valid, isValidDNSName(tcase.name))
		})
	}
}

func TestIsValidIPAddress(t *testing.T) {
	type testCase struct {
		name  string
		valid bool
	}

	cases := map[string]testCase{
		"ipv4": {
			name:  "192.168.1.2",
			valid: true,
		},
		"ipv6": {
			name:  "2001:db8::1",
			valid: true,
		},
		"ipv4-mapped-ipv6": {
			name:  "::ffff:192.0.2.128",
			valid: true,
		},
		"invalid": {
			name:  "foo",
			valid: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.valid, isValidIPAddress(tcase.name))
		})
	}
}

// TestIsValidPort tests both physical and virtual port validation using
// the same cases to ensure same coverage.
func TestIsValidPort(t *testing.T) {
	type testCase struct {
		port          int
		validVirtual  bool
		validPhysical bool
	}

	cases := map[string]testCase{
		"negative": {
			port:          -1,
			validPhysical: false,
			validVirtual:  false,
		},
		"zero": {
			port:          0,
			validPhysical: false,
			validVirtual:  true,
		},
		"min": {
			port:          1,
			validPhysical: true,
			validVirtual:  true,
		},
		"8080": {
			port:          8080,
			validPhysical: true,
			validVirtual:  true,
		},
		"max": {
			port:          65535,
			validPhysical: true,
			validVirtual:  true,
		},
		"above-max": {
			port:          65536,
			validPhysical: false,
			validVirtual:  false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.validPhysical, isValidPhysicalPortNumber(tcase.port))
			require.Equal(t, tcase.validVirtual, ValidateVirtualPort(tcase.port) == nil)
		})
	}
}

func TestIsValidUnixSocketPath(t *testing.T) {
	type testCase struct {
		name  string
		valid bool
	}

	cases := map[string]testCase{
		"valid": {
			name:  "unix:///foo/bar.sock",
			valid: true,
		},
		"missing-prefix": {
			name:  "/foo/bar.sock",
			valid: false,
		},
		"too-long": {
			name:  fmt.Sprintf("unix://%s/bar.sock", strings.Repeat("/aaaaaaaaaa", 11)),
			valid: false,
		},
		"nul-in-name": {
			name:  "unix:///foo/bar\000sock",
			valid: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tcase.valid, IsValidUnixSocketPath(tcase.name))
		})
	}
}

func TestValidateHost(t *testing.T) {
	type testCase struct {
		name  string
		valid bool
	}

	cases := map[string]testCase{
		"ip-address": {
			name:  "198.18.0.1",
			valid: true,
		},
		"unix-socket": {
			name:  "unix:///foo.sock",
			valid: true,
		},
		"dns-name": {
			name:  "foo.com",
			valid: true,
		},
		"host-empty": {
			name:  "",
			valid: false,
		},
		"host-invalid": {
			name:  "unix:///foo/bar\000sock",
			valid: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateWorkloadHost(tcase.name)
			if tcase.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, errInvalidWorkloadHostFormat{Host: tcase.name}, err)
			}
		})
	}
}

func TestValidateSelector(t *testing.T) {
	type testCase struct {
		selector   *pbcatalog.WorkloadSelector
		allowEmpty bool
		err        error
	}

	cases := map[string]testCase{
		"nil-disallowed": {
			selector:   nil,
			allowEmpty: false,
			err:        resource.ErrEmpty,
		},
		"nil-allowed": {
			selector:   nil,
			allowEmpty: true,
			err:        nil,
		},
		"empty-allowed": {
			selector:   &pbcatalog.WorkloadSelector{},
			allowEmpty: true,
			err:        nil,
		},
		"empty-disallowed": {
			selector:   &pbcatalog.WorkloadSelector{},
			allowEmpty: false,
			err:        resource.ErrEmpty,
		},
		"ok": {
			selector: &pbcatalog.WorkloadSelector{
				Names:    []string{"foo", "bar"},
				Prefixes: []string{"foo", "bar"},
			},
			allowEmpty: false,
			err:        nil,
		},
		"empty-name": {
			selector: &pbcatalog.WorkloadSelector{
				Names:    []string{"", "bar", ""},
				Prefixes: []string{"foo", "bar"},
			},
			allowEmpty: false,
			err: &multierror.Error{
				Errors: []error{
					resource.ErrInvalidListElement{
						Name:    "names",
						Index:   0,
						Wrapped: resource.ErrEmpty,
					},
					resource.ErrInvalidListElement{
						Name:    "names",
						Index:   2,
						Wrapped: resource.ErrEmpty,
					},
				},
			},
		},
		"filter-with-empty-query": {
			selector: &pbcatalog.WorkloadSelector{
				Filter: "garbage.value == zzz",
			},
			allowEmpty: true,
			err: resource.ErrInvalidField{
				Name: "filter",
				Wrapped: errors.New(
					`filter cannot be set unless there is a name or prefix selector`,
				),
			},
		},
		"bad-filter": {
			selector: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo", "bar"},
				Filter:   "garbage.value == zzz",
			},
			allowEmpty: false,
			err: &multierror.Error{
				Errors: []error{
					resource.ErrInvalidField{
						Name: "filter",
						Wrapped: fmt.Errorf(
							`filter "garbage.value == zzz" is invalid: %w`,
							errors.New(`Selector "garbage" is not valid`),
						),
					},
				},
			},
		},
		"good-filter": {
			selector: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"foo", "bar"},
				Filter:   "metadata.zone == west1",
			},
			allowEmpty: false,
			err:        nil,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateSelector(tcase.selector, tcase.allowEmpty)
			if tcase.err == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, tcase.err, err)
			}
		})
	}
}

func TestValidateIPAddress(t *testing.T) {
	// this test does not perform extensive validation of what constitutes
	// an IP address. Instead that is handled in the test for the
	// isValidIPAddress function

	t.Run("empty", func(t *testing.T) {
		require.Equal(t, resource.ErrEmpty, validateIPAddress(""))
	})

	t.Run("invalid", func(t *testing.T) {
		require.Equal(t, errNotIPAddress, validateIPAddress("foo.com"))
	})

	t.Run("ok", func(t *testing.T) {
		require.NoError(t, validateIPAddress("192.168.0.1"))
	})
}

func TestValidatePortName(t *testing.T) {
	type testCase struct {
		name  string
		valid bool
	}

	cases := map[string]testCase{
		"min-length": {
			name:  "a",
			valid: true,
		},
		"max-length": {
			name:  "a1b2c3d4e5f6g7h",
			valid: true,
		},
		"underscore-not-allowed": {
			name:  "has_underscores",
			valid: false,
		},
		"hyphenated": {
			name:  "has-hyphen3",
			valid: true,
		},
		"uppercase-allowed": {
			name:  "UPPERCASE",
			valid: true,
		},
		"numeric-start": {
			name:  "1abc",
			valid: true,
		},
		"numeric-start-with-hypen": {
			name:  "1-abc",
			valid: true,
		},
		"at-least-one-alpha-required": {
			name:  "1234",
			valid: false,
		},
		"hyphen-start-not-allowed": {
			name:  "-abc",
			valid: false,
		},
		"hyphen-end-not-allowed": {
			name:  "abc-",
			valid: false,
		},
		"unicode-not allowed": {
			name:  "abc∑",
			valid: false,
		},
		"too-long": {
			name:  strings.Repeat("a", 16),
			valid: false,
		},
		"missing-name": {
			name:  "",
			valid: false,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidatePortName(tcase.name)
			if tcase.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if tcase.name == "" {
					require.Equal(t, resource.ErrEmpty, err)
				} else {
					require.Equal(t, errNotPortName, err)
				}
			}
		})
	}
}

func TestValidatePortID(t *testing.T) {
	// this test does not perform extensive validation of what constitutes
	// a valid port ID because it is a combination of ValidatePortName and
	// ValidateVirtualPort. In general the criteria is that it must not
	// be empty and must be either a valid DNS label or stringified port
	// number between 1 and 65535. Extensive testing is performed within the
	// tests for those functions.

	t.Run("empty", func(t *testing.T) {
		require.Equal(t, resource.ErrEmpty, ValidateServicePortID(""))
	})

	t.Run("invalid", func(t *testing.T) {
		require.Equal(t, errInvalidPortID, ValidateServicePortID("foo.com"))
	})

	t.Run("invalid", func(t *testing.T) {
		require.Equal(t, errInvalidPortID, ValidateServicePortID("-1"))
	})

	t.Run("invalid", func(t *testing.T) {
		require.Equal(t, errInvalidPortID, ValidateServicePortID("0"))
	})

	t.Run("invalid", func(t *testing.T) {
		require.Equal(t, errInvalidPortID, ValidateServicePortID("65536"))
	})

	t.Run("ok", func(t *testing.T) {
		require.NoError(t, ValidateServicePortID("http"))
	})

	t.Run("ok", func(t *testing.T) {
		require.NoError(t, ValidateServicePortID("8080"))
	})
}

func TestValidateWorkloadAddress(t *testing.T) {
	type testCase struct {
		addr        *pbcatalog.WorkloadAddress
		ports       map[string]*pbcatalog.WorkloadPort
		validateErr func(*testing.T, error)
	}

	cases := map[string]testCase{
		"invalid-host": {
			addr: &pbcatalog.WorkloadAddress{
				Host: "-blarg",
			},
			ports: map[string]*pbcatalog.WorkloadPort{},
			validateErr: func(t *testing.T, err error) {
				var actual resource.ErrInvalidField
				require.ErrorAs(t, err, &actual)
				require.Equal(t, "host", actual.Name)
			},
		},
		"unix-socket-multiport-explicit": {
			addr: &pbcatalog.WorkloadAddress{
				Host:  "unix:///foo.sock",
				Ports: []string{"foo", "bar"},
			},
			ports: map[string]*pbcatalog.WorkloadPort{
				"foo": {},
				"bar": {},
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errUnixSocketMultiport)
			},
		},
		"unix-socket-multiport-implicit": {
			addr: &pbcatalog.WorkloadAddress{
				Host: "unix:///foo.sock",
			},
			ports: map[string]*pbcatalog.WorkloadPort{
				"foo": {},
				"bar": {},
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errUnixSocketMultiport)
			},
		},

		"unix-socket-singleport": {
			addr: &pbcatalog.WorkloadAddress{
				Host:  "unix:///foo.sock",
				Ports: []string{"foo"},
			},
			ports: map[string]*pbcatalog.WorkloadPort{
				"foo": {},
				"bar": {},
			},
		},
		"invalid-port-reference": {
			addr: &pbcatalog.WorkloadAddress{
				Host:  "198.18.0.1",
				Ports: []string{"foo"},
			},
			ports: map[string]*pbcatalog.WorkloadPort{
				"http": {},
			},
			validateErr: func(t *testing.T, err error) {
				var actual errInvalidPortReference
				require.ErrorAs(t, err, &actual)
				require.Equal(t, "foo", actual.Name)
			},
		},
		"ok": {
			addr: &pbcatalog.WorkloadAddress{
				Host:     "198.18.0.1",
				Ports:    []string{"http", "grpc"},
				External: true,
			},
			ports: map[string]*pbcatalog.WorkloadPort{
				"http": {},
				"grpc": {},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateWorkloadAddress(tcase.addr, tcase.ports)
			if tcase.validateErr != nil {
				require.Error(t, err)
				tcase.validateErr(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDNSPolicy(t *testing.T) {
	type testCase struct {
		policy      *pbcatalog.DNSPolicy
		validateErr func(*testing.T, error)
	}

	cases := map[string]testCase{
		"invalid-passing-weight-too-high": {
			policy: &pbcatalog.DNSPolicy{
				Weights: &pbcatalog.Weights{
					Passing: 1000000,
				},
			},
			validateErr: func(t *testing.T, err error) {
				var actual resource.ErrInvalidField
				require.ErrorAs(t, err, &actual)
				require.Equal(t, "passing", actual.Name)
				require.ErrorIs(t, err, errDNSPassingWeightOutOfRange)
			},
		},
		"invalid-passing-weight-too-low": {
			policy: &pbcatalog.DNSPolicy{
				Weights: &pbcatalog.Weights{
					Passing: 0,
				},
			},
			validateErr: func(t *testing.T, err error) {
				var actual resource.ErrInvalidField
				require.ErrorAs(t, err, &actual)
				require.Equal(t, "passing", actual.Name)
				require.ErrorIs(t, err, errDNSPassingWeightOutOfRange)
			},
		},
		"invalid-warning-weight": {
			policy: &pbcatalog.DNSPolicy{
				Weights: &pbcatalog.Weights{
					Passing: 1,
					Warning: 1000000,
				},
			},
			validateErr: func(t *testing.T, err error) {
				var actual resource.ErrInvalidField
				require.ErrorAs(t, err, &actual)
				require.Equal(t, "warning", actual.Name)
				require.ErrorIs(t, err, errDNSWarningWeightOutOfRange)
			},
		},
		"ok": {
			policy: &pbcatalog.DNSPolicy{
				Weights: &pbcatalog.Weights{
					Passing: 3,
					Warning: 0,
				},
			},
		},
		"nil ok": {
			policy: &pbcatalog.DNSPolicy{},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateDNSPolicy(tcase.policy)
			if tcase.validateErr != nil {
				require.Error(t, err)
				tcase.validateErr(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateReferenceType(t *testing.T) {
	allowedType := &pbresource.Type{
		Group:        "foo",
		GroupVersion: "v1",
		Kind:         "Bar",
	}

	type testCase struct {
		check *pbresource.Type
		err   bool
	}

	cases := map[string]testCase{
		"ok": {
			check: allowedType,
			err:   false,
		},
		"group-mismatch": {
			check: &pbresource.Type{
				Group:        "food",
				GroupVersion: "v1",
				Kind:         "Bar",
			},
			err: true,
		},
		"group-version-mismatch": {
			check: &pbresource.Type{
				Group:        "foo",
				GroupVersion: "v2",
				Kind:         "Bar",
			},
			err: true,
		},
		"kind-mismatch": {
			check: &pbresource.Type{
				Group:        "foo",
				GroupVersion: "v1",
				Kind:         "Baz",
			},
			err: true,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateReferenceType(allowedType, tcase.check)
			if tcase.err {
				require.Equal(t, resource.ErrInvalidReferenceType{AllowedType: allowedType}, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateReferenceTenancy(t *testing.T) {
	allowedTenancy := &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
	}

	type testCase struct {
		check *pbresource.Tenancy
		err   bool
	}

	cases := map[string]testCase{
		"ok": {
			check: allowedTenancy,
			err:   false,
		},
		"partition-mismatch": {
			check: &pbresource.Tenancy{
				Partition: "food",
				Namespace: "default",
			},
			err: true,
		},
		"namespace-mismatch": {
			check: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "v2",
			},
			err: true,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateReferenceTenancy(allowedTenancy, tcase.check)
			if tcase.err {
				require.Equal(t, resource.ErrReferenceTenancyNotEqual, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateReference(t *testing.T) {
	allowedTenancy := &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
	}

	allowedType := pbcatalog.WorkloadType

	type testCase struct {
		check *pbresource.ID
		err   error
	}

	cases := map[string]testCase{
		"ok": {
			check: &pbresource.ID{
				Type:    allowedType,
				Tenancy: allowedTenancy,
				Name:    "foo",
			},
		},
		"type-err": {
			check: &pbresource.ID{
				Type:    pbcatalog.NodeType,
				Tenancy: allowedTenancy,
				Name:    "foo",
			},
			err: resource.ErrInvalidReferenceType{AllowedType: allowedType},
		},
		"tenancy-mismatch": {
			check: &pbresource.ID{
				Type: allowedType,
				Tenancy: &pbresource.Tenancy{
					Partition: "foo",
					Namespace: "bar",
				},
				Name: "foo",
			},
			err: resource.ErrReferenceTenancyNotEqual,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateReference(allowedType, allowedTenancy, tcase.check)
			if tcase.err != nil {
				require.ErrorIs(t, err, tcase.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
