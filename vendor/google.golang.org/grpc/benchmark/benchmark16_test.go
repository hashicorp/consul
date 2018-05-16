// +build go1.6,!go1.7

/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package benchmark

import (
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/benchmark/stats"
)

func BenchmarkClientStreamc1(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 1, 1, 1, false})
}

func BenchmarkClientStreamc8(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 8, 1, 1, false})
}

func BenchmarkClientStreamc64(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 64, 1, 1, false})
}

func BenchmarkClientStreamc512(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 512, 1, 1, false})
}
func BenchmarkClientUnaryc1(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 1, 1, 1, false})
}

func BenchmarkClientUnaryc8(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 8, 1, 1, false})
}

func BenchmarkClientUnaryc64(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 64, 1, 1, false})
}

func BenchmarkClientUnaryc512(b *testing.B) {
	grpc.EnableTracing = true
	runStream(b, stats.Features{"", true, 0, 0, 0, 512, 1, 1, false})
}

func BenchmarkClientStreamNoTracec1(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 1, 1, 1, false})
}

func BenchmarkClientStreamNoTracec8(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 8, 1, 1, false})
}

func BenchmarkClientStreamNoTracec64(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 64, 1, 1, false})
}

func BenchmarkClientStreamNoTracec512(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 512, 1, 1, false})
}
func BenchmarkClientUnaryNoTracec1(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 1, 1, 1, false})
}

func BenchmarkClientUnaryNoTracec8(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 8, 1, 1, false})
}

func BenchmarkClientUnaryNoTracec64(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 64, 1, 1, false})
}

func BenchmarkClientUnaryNoTracec512(b *testing.B) {
	grpc.EnableTracing = false
	runStream(b, stats.Features{"", false, 0, 0, 0, 512, 1, 1, false})
	runStream(b, stats.Features{"", false, 0, 0, 0, 512, 1, 1, false})
}

func TestMain(m *testing.M) {
	os.Exit(stats.RunTestMain(m))
}
