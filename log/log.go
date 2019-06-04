package log

import (
	"errors"
	"log"
)

// Func function definition.
type Func func(string, ...interface{})

var (
	infof = func(format string, v ...interface{}) {
		log.Printf("INFO: "+format, v...)
	}
	warnf = func(format string, v ...interface{}) {
		log.Printf("WARN: "+format, v...)
	}
	errorf = func(format string, v ...interface{}) {
		log.Printf("ERROR: "+format, v...)
	}
)

// Setup allows for setting up custom loggers.
func Setup(inf, waf, erf Func) error {
	if inf == nil {
		return errors.New("info log function is nil")
	}
	if waf == nil {
		return errors.New("warn log function is nil")
	}
	if erf == nil {
		return errors.New("error log function is nil")
	}
	infof = inf
	warnf = waf
	errorf = erf
	return nil
}

// Infof provides log info capabilities.
func Infof(format string, v ...interface{}) {
	infof(format, v...)
}

// Warnf provides log warn capabilities.
func Warnf(format string, v ...interface{}) {
	warnf(format, v...)
}

// Errorf provides log error capabilities.
func Errorf(format string, v ...interface{}) {
	errorf(format, v...)
}
