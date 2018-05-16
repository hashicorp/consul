package stats

import (
	"context"
)

type Executor func(func(ctx context.Context))

func DefaultExecutor(handler func(ctx context.Context)) {
	go func() {
		handler(context.Background())
	}()
}