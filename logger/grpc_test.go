package logger

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/grpclog"
)

func TestGRPCLogger(t *testing.T) {
	var out bytes.Buffer
	// No flags so we don't have to include date/time in expected output
	logger := log.New(&out, "", 0)
	grpclog.SetLoggerV2(NewGRPCLogger(&Config{LogLevel: "TRACE"}, logger))

	// All of these should output something
	grpclog.Info("Info,")
	grpclog.Infoln("Infoln")
	grpclog.Infof("Infof: %d\n", 1)

	grpclog.Warning("Warning,")
	grpclog.Warningln("Warningln")
	grpclog.Warningf("Warningf: %d\n", 1)

	grpclog.Error("Error,")
	grpclog.Errorln("Errorln")
	grpclog.Errorf("Errorf: %d\n", 1)

	// Fatal tests are hard... assume they are good!
	expect := `[INFO] Info,
[INFO] Infoln
[INFO] Infof: 1
[WARN] Warning,
[WARN] Warningln
[WARN] Warningf: 1
[ERR] Error,
[ERR] Errorln
[ERR] Errorf: 1
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
			// No flags so we don't have to include date/time in expected output
			logger := log.New(&out, "", 0)
			grpclog.SetLoggerV2(NewGRPCLogger(&Config{LogLevel: tt.level}, logger))

			assert.Equal(t, tt.want, grpclog.V(tt.v))
		})
	}

}
