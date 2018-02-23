package plz

import (
	"fmt"
	"github.com/v2pro/plz/countlog"
	"io"
	"runtime"
)

type MultiError []error

func (errs MultiError) Error() string {
	return "multiple errors"
}

func MergeErrors(errs ...error) error {
	var nonNilErrs []error
	for _, err := range errs {
		if err != nil {
			nonNilErrs = append(nonNilErrs, err)
		}
	}
	if len(nonNilErrs) == 0 {
		return nil
	}
	return MultiError(nonNilErrs)
}

func Close(resource io.Closer, properties ...interface{}) error {
	err := resource.Close()
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		closedAt := fmt.Sprintf("%s:%d", file, line)
		properties = append(properties, "err", err)
		countlog.Error("event!close "+closedAt, properties...)
		return err
	}
	return nil
}

func CloseAll(resources []io.Closer, properties ...interface{}) error {
	var errs []error
	for _, resource := range resources {
		err := resource.Close()
		if err != nil {
			_, file, line, _ := runtime.Caller(1)
			closedAt := fmt.Sprintf("%s:%d", file, line)
			properties = append(properties, "err", err)
			countlog.Error("event!CloseAll called from "+closedAt, properties...)
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 && len(properties) > 0 {
		_, file, line, _ := runtime.Caller(1)
		closedAt := fmt.Sprintf("%s:%d", file, line)
		countlog.Debug("event!CloseAll called from "+closedAt, properties...)
	}
	return MergeErrors(errs...)
}

type funcResource struct {
	f func() error
}

func (res funcResource) Close() error {
	return res.f()
}

func WrapCloser(f func() error) io.Closer {
	return &funcResource{f}
}
