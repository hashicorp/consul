// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ports

type troubleShootProtocol interface {
	dailPort(hostPort *hostPort) string
}
