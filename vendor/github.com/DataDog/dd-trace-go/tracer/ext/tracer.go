package ext

import (
	"runtime"
	"strings"
)

const (
	Lang          = "go"
	Interpreter   = runtime.Compiler + "-" + runtime.GOARCH + "-" + runtime.GOOS
	TracerVersion = "v0.5.0"
)

var LangVersion = strings.TrimPrefix(runtime.Version(), Lang)
