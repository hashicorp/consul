// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows || plan9 || nacl
// +build windows plan9 nacl

package logging

import (
	"testing"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/stretchr/testify/require"
)

func TestSyslog_Unsupported(t *testing.T) {
	// the role of the underlying go-syslog library is primarily to wrap the
	// default log/syslog package such that when running against an unsupported
	// OS, a meaningful error is returned, so that's what we'll test here by default.
	s, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, "USER", "consul")
	require.Error(t, err)
	require.Nil(t, s)
}
