package log

import (
	"errors"
	"io"
	"log"
	"os"
)

var (
	logger Logger = &stdLogger{}
)

// Logger interface for providing a custom logger.
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	Writer() io.Writer
}

// Setup allows for setting up a custom logger.
func Setup(l Logger) error {
	if l == nil {
		return errors.New("logger is nil")
	}
	logger = l
	return nil
}

// Printf provides log print capabilities.
func Printf(format string, v ...interface{}) {
	logger.Printf(format, v...)
}

// Println provides log print capabilities.
func Println(v ...interface{}) {
	logger.Println(v...)
}

// Writer returns the log writer.
func Writer() io.Writer {
	return logger.Writer()
}

type stdLogger struct {
}

func (stdLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (stdLogger) Println(v ...interface{}) {
	log.Println(v...)
}

func (stdLogger) Writer() io.Writer {
	return os.Stdout
}
