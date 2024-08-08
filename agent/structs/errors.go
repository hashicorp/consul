// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"strings"
)

const (
	errNoLeader                              = "No cluster leader"
	errNoDCPath                              = "No path to datacenter"
	errDCNotAvailable                        = "Remote DC has no server currently reachable"
	errNoServers                             = "No known Consul servers"
	errNotReadyForConsistentReads            = "Not ready to serve consistent reads"
	errSegmentsNotSupported                  = "Network segments are not supported in this version of Consul"
	errRPCRateExceeded                       = "RPC rate limit exceeded"
	errServiceNotFound                       = "Service not found: "
	errQueryNotFound                         = "Query not found"
	errLeaderNotTracked                      = "Raft leader not found in server lookup mapping"
	errConnectNotEnabled                     = "Connect must be enabled in order to use this endpoint"
	errRateLimited                           = "Rate limit reached, try again later" // Note: we depend on this error message in the gRPC ConnectCA.Sign endpoint (see: isRateLimitError).
	errNotPrimaryDatacenter                  = "not the primary datacenter"
	errStateReadOnly                         = "CA Provider State is read-only"
	errSamenessGroupNotFound                 = "Sameness Group not found"
	errSamenessGroupMustBeDefaultForFailover = "Sameness Group must have DefaultForFailover set to true in order to use this endpoint"
)

var (
	ErrNoLeader                              = errors.New(errNoLeader)
	ErrNoDCPath                              = errors.New(errNoDCPath)
	ErrNoServers                             = errors.New(errNoServers)
	ErrNotReadyForConsistentReads            = errors.New(errNotReadyForConsistentReads)
	ErrSegmentsNotSupported                  = errors.New(errSegmentsNotSupported)
	ErrRPCRateExceeded                       = errors.New(errRPCRateExceeded)
	ErrDCNotAvailable                        = errors.New(errDCNotAvailable)
	ErrQueryNotFound                         = errors.New(errQueryNotFound)
	ErrLeaderNotTracked                      = errors.New(errLeaderNotTracked)
	ErrConnectNotEnabled                     = errors.New(errConnectNotEnabled)
	ErrRateLimited                           = errors.New(errRateLimited) // Note: we depend on this error message in the gRPC ConnectCA.Sign endpoint (see: isRateLimitError).
	ErrNotPrimaryDatacenter                  = errors.New(errNotPrimaryDatacenter)
	ErrStateReadOnly                         = errors.New(errStateReadOnly)
	ErrSamenessGroupNotFound                 = errors.New(errSamenessGroupNotFound)
	ErrSamenessGroupMustBeDefaultForFailover = errors.New(errSamenessGroupMustBeDefaultForFailover)
)

func IsErrNoDCPath(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoDCPath)
}

func IsErrQueryNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), errQueryNotFound)
}

func IsErrNoLeader(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoLeader)
}

func IsErrRPCRateExceeded(err error) bool {
	return err != nil && strings.Contains(err.Error(), errRPCRateExceeded)
}

func IsErrSamenessGroupNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), errSamenessGroupNotFound)
}

func IsErrSamenessGroupMustBeDefaultForFailover(err error) bool {
	return err != nil && strings.Contains(err.Error(), errSamenessGroupMustBeDefaultForFailover)
}
