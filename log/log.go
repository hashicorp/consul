package log

import (
	"errors"
	"log"
)

// Funcf function definition.
type Funcf func(string, ...interface{})

// Funcln function definition.
type Funcln func(...interface{})

var (
	printf = func(format string, v ...interface{}) {
		log.Printf(format, v...)
	}
	println = func(v ...interface{}) {
		log.Println(v...)
	}
)

// Setup allows for setting up custom loggers.
func Setup(p Funcf, pln Funcln) error {
	if p == nil {
		return errors.New("printf log function is nil")
	}
	if pln == nil {
		return errors.New("println log function is nil")
	}
	printf = p
	println = pln
	return nil
}

// Printf provides log print capabilities.
func Printf(format string, v ...interface{}) {
	printf(format, v...)
}

// Println provides log print capabilities.
func Println(v ...interface{}) {
	println(v...)
}
