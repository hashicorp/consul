// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	// 108 characters is the max size that Linux (and probably other OSes) will
	// allow for the length of the Unix socket path.
	maxUnixSocketPathLen = 108
)

var (
	dnsLabelRegex   = `^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`
	dnsLabelMatcher = regexp.MustCompile(dnsLabelRegex)
)

func isValidIPAddress(host string) bool {
	return net.ParseIP(host) != nil
}

func isValidDNSName(host string) bool {
	if len(host) > 256 {
		return false
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if !isValidDNSLabel(label) {
			return false
		}
	}

	return true
}

func isValidDNSLabel(label string) bool {
	if len(label) > 64 {
		return false
	}

	return dnsLabelMatcher.Match([]byte(label))
}

func IsValidUnixSocketPath(host string) bool {
	if len(host) > maxUnixSocketPathLen || !strings.HasPrefix(host, "unix://") || strings.Contains(host, "\000") {
		return false
	}

	return true
}

func validateWorkloadHost(host string) error {
	// Check that the host is empty
	if host == "" {
		return errInvalidWorkloadHostFormat{Host: host}
	}

	// Check if the host represents an IP address, unix socket path or a DNS name
	if !isValidIPAddress(host) && !IsValidUnixSocketPath(host) && !isValidDNSName(host) {
		return errInvalidWorkloadHostFormat{Host: host}
	}

	return nil
}

func ValidateSelector(sel *pbcatalog.WorkloadSelector, allowEmpty bool) error {
	if sel == nil {
		if allowEmpty {
			return nil
		}

		return resource.ErrEmpty
	}

	if len(sel.Names) == 0 && len(sel.Prefixes) == 0 {
		if !allowEmpty {
			return resource.ErrEmpty
		}

		if sel.Filter != "" {
			return resource.ErrInvalidField{
				Name:    "filter",
				Wrapped: errors.New("filter cannot be set unless there is a name or prefix selector"),
			}
		}
		return nil
	}

	var merr error

	// Validate that all the exact match names are non-empty. This is
	// mostly for the sake of not admitting values that should always
	// be meaningless and never actually cause selection of a workload.
	// This is because workloads must have non-empty names.
	for idx, name := range sel.Names {
		if name == "" {
			merr = multierror.Append(merr, resource.ErrInvalidListElement{
				Name:    "names",
				Index:   idx,
				Wrapped: resource.ErrEmpty,
			})
		}
	}

	if err := resource.ValidateMetadataFilter(sel.GetFilter()); err != nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "filter",
			Wrapped: err,
		})
	}

	return merr
}

func validateIPAddress(ip string) error {
	if ip == "" {
		return resource.ErrEmpty
	}

	if !isValidIPAddress(ip) {
		return errNotIPAddress
	}

	return nil
}

func ValidatePortName(name string) error {
	if name == "" {
		return resource.ErrEmpty
	}

	if !isValidDNSLabel(name) {
		return errNotDNSLabel
	}

	return nil
}

func ValidateProtocol(protocol pbcatalog.Protocol) error {
	// enumcover:pbcatalog.Protocol
	switch protocol {
	case pbcatalog.Protocol_PROTOCOL_UNSPECIFIED,
		// means pbcatalog.FailoverMode_FAILOVER_MODE_TCP
		pbcatalog.Protocol_PROTOCOL_TCP,
		pbcatalog.Protocol_PROTOCOL_HTTP,
		pbcatalog.Protocol_PROTOCOL_HTTP2,
		pbcatalog.Protocol_PROTOCOL_GRPC,
		pbcatalog.Protocol_PROTOCOL_MESH:
		return nil
	default:
		return resource.NewConstError(fmt.Sprintf("not a supported enum value: %v", protocol))
	}
}

// validateWorkloadAddress will validate the WorkloadAddress type. This involves validating
// the Host within the workload address and the ports references. For ports references we
// ensure that values in the addresses ports array are present in the set of map keys.
// Additionally for UNIX socket addresses we ensure that they specify only 1 port either
// explicitly in their ports references or implicitly by omitting the references and there
// only being 1 value in the ports map.
func validateWorkloadAddress(addr *pbcatalog.WorkloadAddress, ports map[string]*pbcatalog.WorkloadPort) error {
	var err error

	if hostErr := validateWorkloadHost(addr.Host); hostErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "host",
			Wrapped: hostErr,
		})
	}

	// Ensure that unix sockets reference exactly 1 port. They may also indirectly reference 1 port
	// by the workload having only a single port and omitting any explicit port assignment.
	if IsValidUnixSocketPath(addr.Host) &&
		(len(addr.Ports) > 1 || (len(addr.Ports) == 0 && len(ports) > 1)) {
		err = multierror.Append(err, errUnixSocketMultiport)
	}

	// Check that all referenced ports exist
	for idx, port := range addr.Ports {
		_, found := ports[port]
		if !found {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "ports",
				Index:   idx,
				Wrapped: errInvalidPortReference{Name: port},
			})
		}
	}
	return err
}

func validateReferenceType(allowed *pbresource.Type, check *pbresource.Type) error {
	if allowed.Group == check.Group &&
		allowed.GroupVersion == check.GroupVersion &&
		allowed.Kind == check.Kind {
		return nil
	}

	return resource.ErrInvalidReferenceType{
		AllowedType: allowed,
	}
}

func validateReferenceTenancy(allowed *pbresource.Tenancy, check *pbresource.Tenancy) error {
	if proto.Equal(allowed, check) {
		return nil
	}

	return resource.ErrReferenceTenancyNotEqual
}

func validateReference(allowedType *pbresource.Type, allowedTenancy *pbresource.Tenancy, check *pbresource.ID) error {
	var err error

	// Validate the references type is the allowed type.
	if typeErr := validateReferenceType(allowedType, check.GetType()); typeErr != nil {
		err = multierror.Append(err, typeErr)
	}

	// Validate the references tenancy matches the allowed tenancy.
	if tenancyErr := validateReferenceTenancy(allowedTenancy, check.GetTenancy()); tenancyErr != nil {
		err = multierror.Append(err, tenancyErr)
	}

	return err
}

func validateHealth(health pbcatalog.Health) error {
	// enumcover:pbcatalog.Health
	switch health {
	case pbcatalog.Health_HEALTH_ANY,
		pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE:
		return nil
	default:
		return resource.NewConstError(fmt.Sprintf("not a supported enum value: %v", health))
	}
}

// ValidateLocalServiceRefNoSection ensures the following:
//
// - ref is non-nil
// - type is ServiceType
// - section is empty
// - tenancy is set and partition/namespace are both non-empty
// - peer_name must be "local"
//
// Each possible validation error is wrapped in the wrapErr function before
// being collected in a multierror.Error.
func ValidateLocalServiceRefNoSection(ref *pbresource.Reference, wrapErr func(error) error) error {
	if ref == nil {
		return wrapErr(resource.ErrMissing)
	}

	if !resource.EqualType(ref.Type, pbcatalog.ServiceType) {
		return wrapErr(resource.ErrInvalidField{
			Name: "type",
			Wrapped: resource.ErrInvalidReferenceType{
				AllowedType: pbcatalog.ServiceType,
			},
		})
	}

	var merr error
	if ref.Section != "" {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "section",
			Wrapped: errors.New("section cannot be set here"),
		}))
	}

	if ref.Tenancy == nil {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "tenancy",
			Wrapped: resource.ErrMissing,
		}))
	} else {
		// NOTE: these are Service specific, since that's a Namespace-scoped type.
		if ref.Tenancy.Partition == "" {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name: "tenancy",
				Wrapped: resource.ErrInvalidField{
					Name:    "partition",
					Wrapped: resource.ErrEmpty,
				},
			}))
		}
		if ref.Tenancy.Namespace == "" {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name: "tenancy",
				Wrapped: resource.ErrInvalidField{
					Name:    "namespace",
					Wrapped: resource.ErrEmpty,
				},
			}))
		}
		if ref.Tenancy.PeerName != "local" {
			merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
				Name: "tenancy",
				Wrapped: resource.ErrInvalidField{
					Name:    "peer_name",
					Wrapped: errors.New(`must be set to "local"`),
				},
			}))
		}
	}

	if ref.Name == "" {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "name",
			Wrapped: resource.ErrMissing,
		}))
	}

	return merr
}
