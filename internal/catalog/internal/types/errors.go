// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
)

var (
	errNotDNSLabel                = errors.New(fmt.Sprintf("value must match regex: %s", dnsLabelRegex))
	errNotIPAddress               = errors.New("value is not a valid IP address")
	errUnixSocketMultiport        = errors.New("Unix socket address references more than one port")
	errInvalidPhysicalPort        = errors.New("port number is outside the range 1 to 65535")
	errInvalidVirtualPort         = errors.New("port number is outside the range 0 to 65535")
	errDNSWarningWeightOutOfRange = errors.New("DNS warning weight is outside the range 0 to 65535")
	errDNSPassingWeightOutOfRange = errors.New("DNS passing weight is outside of the range 1 to 65535")
	errLocalityZoneNoRegion       = errors.New("locality region cannot be empty if the zone is set")
	errInvalidHealth              = errors.New("health status must be one of: passing, warning, critical or maintenance")
)

type errInvalidWorkloadHostFormat struct {
	Host string
}

func (err errInvalidWorkloadHostFormat) Error() string {
	return fmt.Sprintf("%q is not an IP address, Unix socket path or a DNS name.", err.Host)
}

type errInvalidNodeHostFormat struct {
	Host string
}

func (err errInvalidNodeHostFormat) Error() string {
	return fmt.Sprintf("%q is not an IP address or a DNS name.", err.Host)
}

type errInvalidPortReference struct {
	Name string
}

func (err errInvalidPortReference) Error() string {
	return fmt.Sprintf("port with name %q has not been defined", err.Name)
}

type errVirtualPortReused struct {
	Index int
	Value uint32
}

func (err errVirtualPortReused) Error() string {
	return fmt.Sprintf("virtual port %d was previously assigned at index %d", err.Value, err.Index)
}

type errTooMuchMesh struct {
	Ports []string
}

func (err errTooMuchMesh) Error() string {
	return fmt.Sprintf("protocol \"mesh\" was specified in more than 1 port: %+v", err.Ports)
}
