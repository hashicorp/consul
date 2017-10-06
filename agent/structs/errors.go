package structs

import (
	"errors"
	"strings"
)

const (
	errNoLeader                   = "No cluster leader"
	errNoDCPath                   = "No path to datacenter"
	errNoServers                  = "No known Consul servers"
	errNotReadyForConsistentReads = "Not ready to serve consistent reads"
	errSegmentsNotSupported       = "Network segments are not supported in this version of Consul"
	errRPCRateExceeded            = "RPC rate limit exceeded"
)

var (
	ErrNoLeader                   = errors.New(errNoLeader)
	ErrNoDCPath                   = errors.New(errNoDCPath)
	ErrNoServers                  = errors.New(errNoServers)
	ErrNotReadyForConsistentReads = errors.New(errNotReadyForConsistentReads)
	ErrSegmentsNotSupported       = errors.New(errSegmentsNotSupported)
	ErrRPCRateExceeded            = errors.New(errRPCRateExceeded)
)

func IsErrNoLeader(err error) bool {
	return err != nil && strings.Contains(err.Error(), errNoLeader)
}

func IsErrRPCRateExceeded(err error) bool {
	return err != nil && strings.Contains(err.Error(), errRPCRateExceeded)
}
