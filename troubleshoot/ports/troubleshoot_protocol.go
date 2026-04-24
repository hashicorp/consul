// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ports

type troubleShootProtocol interface {
	dialPort(hostPort *hostPort) error
}
