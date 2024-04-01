// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

// GRPCLogger wrapps a hclog.Logger and implements the grpclog.LoggerV2 interface
// allowing gRPC servers to log to the standard Consul logger.
type GRPCLogger struct {
	level  string
	logger hclog.Logger
}

// NewGRPCLogger creates a grpclog.LoggerV2 that will output to the supplied
// logger with Severity/Verbosity level appropriate for the given config.
//
// Note that grpclog has Info, Warning, Error, Fatal severity levels AND integer
// verbosity levels for additional info. Verbose logs in hclog are always DEBUG
// severity so we map Info,V0 to INFO, Info,V1 to DEBUG, and Info,V>1 to TRACE.
func NewGRPCLogger(logLevel string, logger hclog.Logger) *GRPCLogger {
	return &GRPCLogger{
		level:  logLevel,
		logger: logger,
	}
}

// Info implements grpclog.LoggerV2
func (g *GRPCLogger) Info(args ...interface{}) {
	// gRPC's INFO level is more akin to Consul's TRACE level
	g.logger.Trace(fmt.Sprint(args...))
}

// Infoln implements grpclog.LoggerV2
func (g *GRPCLogger) Infoln(args ...interface{}) {
	g.Info(fmt.Sprint(args...))
}

// Infof implements grpclog.LoggerV2
func (g *GRPCLogger) Infof(format string, args ...interface{}) {
	g.Info(fmt.Sprintf(format, args...))
}

// Warning implements grpclog.LoggerV2
func (g *GRPCLogger) Warning(args ...interface{}) {
	g.logger.Warn(fmt.Sprint(args...))
}

// Warningln implements grpclog.LoggerV2
func (g *GRPCLogger) Warningln(args ...interface{}) {
	g.Warning(fmt.Sprint(args...))
}

// Warningf implements grpclog.LoggerV2
func (g *GRPCLogger) Warningf(format string, args ...interface{}) {
	g.Warning(fmt.Sprintf(format, args...))
}

// Error implements grpclog.LoggerV2
func (g *GRPCLogger) Error(args ...interface{}) {
	g.logger.Error(fmt.Sprint(args...))
}

// Errorln implements grpclog.LoggerV2
func (g *GRPCLogger) Errorln(args ...interface{}) {
	g.Error(fmt.Sprint(args...))
}

// Errorf implements grpclog.LoggerV2
func (g *GRPCLogger) Errorf(format string, args ...interface{}) {
	g.Error(fmt.Sprintf(format, args...))
}

// Fatal implements grpclog.LoggerV2
func (g *GRPCLogger) Fatal(args ...interface{}) {
	g.logger.Error(fmt.Sprint(args...))
}

// Fatalln implements grpclog.LoggerV2
func (g *GRPCLogger) Fatalln(args ...interface{}) {
	g.Fatal(fmt.Sprint(args...))
}

// Fatalf implements grpclog.LoggerV2
func (g *GRPCLogger) Fatalf(format string, args ...interface{}) {
	g.Fatal(fmt.Sprintf(format, args...))
}

// V implements grpclog.LoggerV2
func (g *GRPCLogger) V(l int) bool {
	switch g.level {
	case "TRACE":
		// Enable ALL the verbosity!
		return true
	case "DEBUG":
		return l < 2
	case "INFO":
		return l < 1
	default:
		return false
	}
}
