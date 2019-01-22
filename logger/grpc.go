package logger

import (
	"fmt"
	"log"
)

// GRPCLogger wrapps a *log.Logger and implements the grpclog.LoggerV2 interface
// allowing gRPC servers to log to the standard Consul logger.
type GRPCLogger struct {
	level string
	l     *log.Logger
}

// NewGRPCLogger creates a grpclog.LoggerV2 that will output to the supplied
// logger with Severity/Verbosity level appropriate for the given config.
//
// Note that grpclog has Info, Warning, Error, Fatal severity levels AND integer
// verbosity levels for additional info. Verbose logs in glog are always INFO
// severity so we map Info,V0 to INFO, Info,V1 to DEBUG, and Info,V>1 to TRACE.
func NewGRPCLogger(config *Config, logger *log.Logger) *GRPCLogger {
	return &GRPCLogger{
		level: config.LogLevel,
		l:     logger,
	}
}

// Info implements grpclog.LoggerV2
func (g *GRPCLogger) Info(args ...interface{}) {
	args = append([]interface{}{"[INFO] "}, args...)
	g.l.Print(args...)
}

// Infoln implements grpclog.LoggerV2
func (g *GRPCLogger) Infoln(args ...interface{}) {
	g.Info(fmt.Sprintln(args...))
}

// Infof implements grpclog.LoggerV2
func (g *GRPCLogger) Infof(format string, args ...interface{}) {
	g.Info(fmt.Sprintf(format, args...))
}

// Warning implements grpclog.LoggerV2
func (g *GRPCLogger) Warning(args ...interface{}) {
	args = append([]interface{}{"[WARN] "}, args...)
	g.l.Print(args...)
}

// Warningln implements grpclog.LoggerV2
func (g *GRPCLogger) Warningln(args ...interface{}) {
	g.Warning(fmt.Sprintln(args...))
}

// Warningf implements grpclog.LoggerV2
func (g *GRPCLogger) Warningf(format string, args ...interface{}) {
	g.Warning(fmt.Sprintf(format, args...))
}

// Error implements grpclog.LoggerV2
func (g *GRPCLogger) Error(args ...interface{}) {
	args = append([]interface{}{"[ERR] "}, args...)
	g.l.Print(args...)
}

// Errorln implements grpclog.LoggerV2
func (g *GRPCLogger) Errorln(args ...interface{}) {
	g.Error(fmt.Sprintln(args...))
}

// Errorf implements grpclog.LoggerV2
func (g *GRPCLogger) Errorf(format string, args ...interface{}) {
	g.Error(fmt.Sprintf(format, args...))
}

// Fatal implements grpclog.LoggerV2
func (g *GRPCLogger) Fatal(args ...interface{}) {
	args = append([]interface{}{"[ERR] "}, args...)
	g.l.Fatal(args...)
}

// Fatalln implements grpclog.LoggerV2
func (g *GRPCLogger) Fatalln(args ...interface{}) {
	g.Fatal(fmt.Sprintln(args...))
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
