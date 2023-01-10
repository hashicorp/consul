package hcp

import (
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
)

func formatDuration(d time.Duration) string {
	protoDuration := durationpb.New(d)
	durByte, err := protojson.Marshal(protoDuration)
	if err != nil {
		return "0s"
	}
	return strings.Trim(string(durByte), `"`)
}
