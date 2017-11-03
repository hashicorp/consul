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

package grpc

import (
	"testing"

	"google.golang.org/grpc/resolver"
)

func TestParseTarget(t *testing.T) {
	for _, test := range []resolver.Target{
		{"", "", ""},
		{"a", "", ""},
		{"", "a", ""},
		{"", "", "a"},
		{"a", "b", ""},
		{"a", "", "b"},
		{"", "a", "b"},
		{"a", "b", "c"},
		{"dns", "a.server.com", "google.com"},
		{"dns", "a.server.com", "google.com"},
		{"dns", "a.server.com", "google.com/?a=b"},
	} {
		str := test.Scheme + "://" + test.Authority + "/" + test.Endpoint
		got := parseTarget(str)
		if got != test {
			t.Errorf("parseTarget(%q) = %v, want %v", str, got, test)
		}
	}
}
