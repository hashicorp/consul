package main

import (
	"context"
	"fmt"
	"github.com/v2pro/plz"
	"github.com/v2pro/plz/countlog"
	"os"
	"time"
)

func main() {
	plz.PlugAndPlay()
	ctx := context.WithValue(context.Background(), "traceId", "abcd")
	//err := doSomething(ctx)
	//countlog.TraceCall("callee!main.doSomething", err, "ctx", ctx)
	doZ(countlog.Ctx(ctx))
}

func doX(ctx context.Context) error {
	file, err := os.OpenFile("/tmp/my-dir/abc", os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write([]byte("hello"))
	if err != nil {
		return err
	}
	return nil
}

func doA(ctx context.Context) error {
	file, err := os.OpenFile("/tmp/my-dir/abc", os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()
	_, err = file.Write([]byte("hello"))
	if err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}
	return nil
}

func doY(ctx context.Context) error {
	defer func() {
		recovered := recover()
		if recovered != nil {
			countlog.Fatal("event!doY.panic",
				"err", recovered)
		}
	}()
	start := time.Now()
	file, err := os.OpenFile("/tmp/my-dir/abc", os.O_RDWR, 0666)
	if err != nil {
		countlog.Error("event!metric",
			"callee", "ioutil.WriteFile", "ctx", ctx, "latency", time.Since(start))
		countlog.Error("event!doY.failed to open file", "err", err)
		return err
	}
	countlog.Trace("event!metric",
		"callee", "ioutil.WriteFile", "ctx", ctx, "latency", time.Since(start))
	defer func() {
		err = file.Close()
		if err != nil {
			countlog.Error("event!doY.failed to close file", "err", err)
		}
	}()
	_, err = file.Write([]byte("hello"))
	if err != nil {
		return err
	}
	return nil
}

func doZ(ctx *countlog.Context) error {
	defer func() {
		countlog.LogPanic(recover())
	}()
	path := "/tmp/abc"
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	// add event! prefix to make log message more findable
	ctx.TraceCall("event!doZ os.OpenFile", err)
	if err != nil {
		return err
	}
	defer plz.Close(file)
	_, err = file.Write([]byte("hello"))
	// without event! prefix also works,
	// but event name must not be dynamic formatted string
	// TraceCall will also generate new error object with more context
	// return this error will give user a better clue about what happened
	err = ctx.TraceCall("doZ write file {path}", err,
		"path", path)
	if err != nil {
		return err
	}
	return nil
}
