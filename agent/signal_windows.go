// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package agent

import (
	"os"
)

var forwardSignals = []os.Signal{os.Interrupt}
