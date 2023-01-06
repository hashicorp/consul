package hcp

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

func formatDuration(d time.Duration) string {
	protoDuration := durationpb.New(d)
	dur := fmt.Sprintf("%d.%.9d", protoDuration.Seconds, protoDuration.Nanos)
	// remove trailing 0's
	dur = strings.TrimSuffix(dur, "000")
	dur = strings.TrimSuffix(dur, "000")
	dur = strings.TrimSuffix(dur, ".000")
	// add trailing s
	return dur + "s"
}
