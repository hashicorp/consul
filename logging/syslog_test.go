// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris
// +build linux darwin dragonfly freebsd netbsd openbsd solaris

package logging

import (
	"testing"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/stretchr/testify/require"
)

// testSyslogFunc is a wrapper for injecting WriteLevel functionality into a Syslogger
// implementation; it gives us a way to inject test assertions into a SyslogWrapper.
type testSyslogWriteLevelFunc func(p gsyslog.Priority, m []byte) error

// Write is a no-op
func (f testSyslogWriteLevelFunc) Write(p []byte) (int, error) {
	return 0, nil
}

// WriteLevel is a wrapper for the identity to be called
func (f testSyslogWriteLevelFunc) WriteLevel(p gsyslog.Priority, m []byte) error {
	return f(p, m)
}

// Close is a no-op
func (f testSyslogWriteLevelFunc) Close() error {
	return nil
}

func TestSyslog_Wrapper(t *testing.T) {
	expectedMsg := []byte("test message")

	// generator for a writer that expects to be written with expectedPriority
	writerForPriority := func(expectedPriority gsyslog.Priority) testSyslogWriteLevelFunc {
		return func(p gsyslog.Priority, m []byte) error {
			require.Equal(t, expectedPriority, p)
			require.Equal(t, expectedMsg, m)
			return nil
		}
	}
	// [TRACE] is mapped to the LOG_DEBUG priority
	debugw := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_DEBUG)}
	_, err := debugw.Write([]byte("[TRACE]   " + string(expectedMsg)))
	require.NoError(t, err)

	// [DEBUG] is mapped to the LOG_INFO priority
	infow := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_INFO)}
	_, err = infow.Write([]byte("[DEBUG]     " + string(expectedMsg)))
	require.NoError(t, err)

	// [INFO] is mapped to the LOG_NOTICE priority
	noticew := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_NOTICE)}
	_, err = noticew.Write([]byte("[INFO]     " + string(expectedMsg)))
	require.NoError(t, err)

	// [WARN] is mapped to the LOG_WARNING priority
	warnw := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_WARNING)}
	_, err = warnw.Write([]byte("[WARN]     " + string(expectedMsg)))
	require.NoError(t, err)

	// [ERROR] is mapped to the LOG_ERR priority
	errorw := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_ERR)}
	_, err = errorw.Write([]byte("[ERROR]        " + string(expectedMsg)))
	require.NoError(t, err)

	// [CRIT] is mapped to the LOG_CRIT priority
	critw := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_CRIT)}
	_, err = critw.Write([]byte("[CRIT]  " + string(expectedMsg)))
	require.NoError(t, err)

	// unknown levels are written with LOG_NOTICE priority
	defaultw := &SyslogWrapper{l: writerForPriority(gsyslog.LOG_NOTICE)}
	_, err = defaultw.Write([]byte("[INVALID]     " + string(expectedMsg)))
	require.NoError(t, err)
}
