package main

import (
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/concurrent"
	"time"
	"errors"
)

func main() {
	concurrent.GlobalUnboundedExecutor.Go(func(ctx *countlog.Context) {
		ctx.Add("traceId", "axkenfppkl")
		timer := countlog.InfoTimer()
		ctx.SuppressLevelsBelow(countlog.LevelInfo)
		req := "request 111"
		err := processRequest(ctx, req)
		ctx.LogAccess("process request", err,
			"request", req,
			"timer", timer)
	})
	time.Sleep(time.Second)
}

func processRequest(ctx *countlog.Context, request string) error {
	ctx.Trace("something minor")
	ctx.Add("userId", "111")
	ctx.Info("calculated game scores", "score", []int{1, 2, 3})
	return errors.New("failed")
}
