// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package lifecycle

import "time"

// Logger is the logging interface the Exchange uses. hclog.Logger satisfies it.
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// Metrics is the metrics interface the Exchange uses. Names are relative; the
// adapter adds any service prefix.
type Metrics interface {
	SetGauge(name []string, value float32, labels map[string]string)
	IncrCounter(name []string, value float32, labels map[string]string)
}

// Clock abstracts time for tests.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is returned by Clock.AfterFunc.
type Timer interface {
	Stop() bool
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...interface{}) {}
func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Warn(string, ...interface{})  {}
func (noopLogger) Error(string, ...interface{}) {}

type noopMetrics struct{}

func (noopMetrics) SetGauge([]string, float32, map[string]string)    {}
func (noopMetrics) IncrCounter([]string, float32, map[string]string) {}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) AfterFunc(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f)
}
