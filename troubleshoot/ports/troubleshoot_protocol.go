// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package ports

type troubleShootProtocol interface {
	dialPort(hostPort *hostPort) error
}
