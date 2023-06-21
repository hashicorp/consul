// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package logging

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/grpclog"
)

func TestGRPCLogger(t *testing.T) {
	var out bytes.Buffer
	// Use a placeholder value for TimeFormat so we don't care about dates/times
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     &out,
		TimeFormat: "timeformat",
	})
	grpclog.SetLoggerV2(NewGRPCLogger("TRACE", logger))

	// All of these should output something
	grpclog.Info("Info,")
	grpclog.Infoln("Infoln")
	grpclog.Infof("Infof: %d", 1)

	grpclog.Warning("Warning,")
	grpclog.Warningln("Warningln")
	grpclog.Warningf("Warningf: %d", 1)

	grpclog.Error("Error,")
	grpclog.Errorln("Errorln")
	grpclog.Errorf("Errorf: %d", 1)

	// Fatal tests are hard... assume they are good!
	expect := `timeformat [TRACE] Info,
timeformat [TRACE] Infoln
timeformat [TRACE] Infof: 1
timeformat [WARN]  Warning,
timeformat [WARN]  Warningln
timeformat [WARN]  Warningf: 1
timeformat [ERROR] Error,
timeformat [ERROR] Errorln
timeformat [ERROR] Errorf: 1
`

	require.Equal(t, expect, out.String())
}

func TestGRPCLogger_V(t *testing.T) {

	tests := []struct {
		level string
		v     int
		want  bool
	}{
		{"ERR", -1, false},
		{"ERR", 0, false},
		{"ERR", 1, false},
		{"ERR", 2, false},
		{"ERR", 3, false},
		{"WARN", -1, false},
		{"WARN", 0, false},
		{"WARN", 1, false},
		{"WARN", 2, false},
		{"WARN", 3, false},
		{"INFO", -1, true},
		{"INFO", 0, true},
		{"INFO", 1, false},
		{"INFO", 2, false},
		{"INFO", 3, false},
		{"DEBUG", -1, true},
		{"DEBUG", 0, true},
		{"DEBUG", 1, true},
		{"DEBUG", 2, false},
		{"DEBUG", 3, false},
		{"TRACE", -1, true},
		{"TRACE", 0, true},
		{"TRACE", 1, true},
		{"TRACE", 2, true},
		{"TRACE", 3, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s,%d", tt.level, tt.v), func(t *testing.T) {
			var out bytes.Buffer
			logger := hclog.New(&hclog.LoggerOptions{
				Name:   t.Name(),
				Level:  hclog.Trace,
				Output: &out,
			})
			grpclog.SetLoggerV2(NewGRPCLogger(tt.level, logger))

			assert.Equal(t, tt.want, grpclog.V(tt.v))
		})
	}
}
