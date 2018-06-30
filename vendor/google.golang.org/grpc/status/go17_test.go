// +build go1.7

/*
 *
 * Copyright 2018 gRPC authors.
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

package status

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestFromStdContextError(t *testing.T) {
	testCases := []struct {
		in   error
		want *Status
	}{
		{in: context.DeadlineExceeded, want: New(codes.DeadlineExceeded, context.DeadlineExceeded.Error())},
		{in: context.Canceled, want: New(codes.Canceled, context.Canceled.Error())},
	}
	for _, tc := range testCases {
		got := FromContextError(tc.in)
		if got.Code() != tc.want.Code() || got.Message() != tc.want.Message() {
			t.Errorf("FromContextError(%v) = %v; want %v", tc.in, got, tc.want)
		}
	}
}
