// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lib

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/yamux"
)

var yamuxStreamClosed = yamux.ErrStreamClosed.Error()
var yamuxSessionShutdown = yamux.ErrSessionShutdown.Error()

// IsErrEOF returns true if we get an EOF error from the socket itself, or
// an EOF equivalent error from yamux.
func IsErrEOF(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}

	errStr := err.Error()
	if strings.Contains(errStr, yamuxStreamClosed) ||
		strings.Contains(errStr, yamuxSessionShutdown) {
		return true
	}

	var serverError rpc.ServerError
	if errors.As(err, &serverError) {
		return strings.HasSuffix(err.Error(), fmt.Sprintf(": %s", io.EOF.Error()))
	}

	return false
}
